package dynamicapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	R "github.com/sagernet/sing-box/route/rule"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

var _ adapter.DynamicManager = (*Server)(nil)

type Server struct {
	ctx           context.Context
	router        adapter.Router
	inbound       adapter.InboundManager
	outbound      adapter.OutboundManager
	logger        log.ContextLogger
	logFactory    log.Factory
	httpServer    *http.Server
	listenAddress string
	secret        string
}

func NewServer(ctx context.Context, logger log.ContextLogger, options option.DynamicAPIOptions) (adapter.DynamicManager, error) {
	r := chi.NewRouter()

	inboundManager := service.FromContext[adapter.InboundManager](ctx)
	outboundManager := service.FromContext[adapter.OutboundManager](ctx)
	routerInstance := service.FromContext[adapter.Router](ctx)
	logFactory := service.FromContext[log.Factory](ctx)

	s := &Server{
		ctx:           ctx,
		router:        routerInstance,
		inbound:       inboundManager,
		outbound:      outboundManager,
		logger:        logger,
		logFactory:    logFactory,
		listenAddress: options.Listen,
		secret:        options.Secret,
		httpServer: &http.Server{
			Addr:    options.Listen,
			Handler: r,
		},
	}

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(authentication(options.Secret))

	// 添加API路由
	r.Route("/api", func(r chi.Router) {
		// 入站API
		r.Route("/inbound", func(r chi.Router) {
			r.Post("/", s.createInbound)
			r.Delete("/{tag}", s.removeInbound)
			r.Get("/", s.listInbounds)
		})

		// 出站API
		r.Route("/outbound", func(r chi.Router) {
			r.Post("/", s.createOutbound)
			r.Delete("/{tag}", s.removeOutbound)
			r.Get("/", s.listOutbounds)
		})

		// 路由规则API
		r.Route("/route", func(r chi.Router) {
			r.Post("/rule", s.createRouteRule)
			r.Delete("/rule/{index}", s.removeRouteRule)
			r.Get("/rules", s.listRouteRules)
		})
	})

	return s, nil
}

func (s *Server) Name() string {
	return "dynamic api server"
}

func (s *Server) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStatePostStart {
		return nil
	}

	listener, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return E.Cause(err, "listen on ", s.listenAddress)
	}

	s.logger.Info("dynamic api server listening at ", listener.Addr())

	go func() {
		err = s.httpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("failed to serve: ", err)
		}
	}()

	return nil
}

func (s *Server) Close() error {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// 修改createInbound方法
func (s *Server) createInbound(w http.ResponseWriter, r *http.Request) {
	// 从请求体中读取原始JSON数据
	body, err := io.ReadAll(r.Body)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无法读取请求体: " + err.Error()})
		return
	}
	defer r.Body.Close()

	// 首先尝试解析整个请求
	var requestMap map[string]interface{}
	if err := json.Unmarshal(body, &requestMap); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无法解析请求: " + err.Error()})
		return
	}

	// 提取tag和type
	tag, tagExists := requestMap["tag"].(string)
	inboundType, typeExists := requestMap["type"].(string)

	if !tagExists || !typeExists {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "tag和type不能为空"})
		return
	}

	// 检查入站是否已存在
	if _, exists := s.inbound.Get(tag); exists {
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]string{"error": "入站已存在: " + tag})
		return
	}

	// 提取options
	var optionsRaw interface{}
	if options, hasOptions := requestMap["options"]; hasOptions {
		optionsRaw = options
	} else {
		// 如果没有options字段，将请求中除了tag和type外的所有字段作为options
		optionsMap := make(map[string]interface{})
		for key, value := range requestMap {
			if key != "tag" && key != "type" {
				optionsMap[key] = value
			}
		}
		optionsRaw = optionsMap
	}

	// 记录日志
	s.logger.Info("创建入站: ", inboundType, "[", tag, "]")

	// 获取入站注册表
	inboundRegistry := service.FromContext[adapter.InboundRegistry](s.ctx)
	if inboundRegistry == nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "入站注册服务不可用"})
		return
	}

	// 创建入站配置对象
	optionsObj, exists := inboundRegistry.CreateOptions(inboundType)
	if !exists {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "不支持的入站类型: " + inboundType})
		return
	}

	// 将原始选项转换为正确的结构体
	optionsJson, err := json.Marshal(optionsRaw)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无法序列化选项: " + err.Error()})
		return
	}

	err = json.Unmarshal(optionsJson, optionsObj)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "选项格式错误: " + err.Error()})
		return
	}

	// 创建入站
	err = s.inbound.Create(s.ctx, s.router, s.logger, tag, inboundType, optionsObj)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "创建入站失败: " + err.Error()})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "入站创建成功",
		"tag":     tag,
		"type":    inboundType,
	})
}

// 移除入站
func (s *Server) removeInbound(w http.ResponseWriter, r *http.Request) {
	tag := chi.URLParam(r, "tag")
	if tag == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "tag不能为空"})
		return
	}

	// 检查入站是否存在
	_, exists := s.inbound.Get(tag)
	if !exists {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": "入站不存在: " + tag})
		return
	}

	// 移除入站
	err := s.inbound.Remove(tag)
	if err != nil {
		if errors.Is(err, os.ErrInvalid) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{"error": "入站不存在: " + tag})
		} else {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, map[string]string{"error": "移除入站失败: " + err.Error()})
		}
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "入站移除成功",
		"tag":     tag,
	})
}

// 列出所有入站
func (s *Server) listInbounds(w http.ResponseWriter, r *http.Request) {
	inbounds := s.inbound.Inbounds()
	var result []map[string]string

	for _, inbound := range inbounds {
		result = append(result, map[string]string{
			"tag":  inbound.Tag(),
			"type": inbound.Type(),
		})
	}

	render.JSON(w, r, map[string]interface{}{
		"success":  true,
		"inbounds": result,
	})
}

// 修改createOutbound方法
func (s *Server) createOutbound(w http.ResponseWriter, r *http.Request) {
	// 从请求体中读取原始JSON数据
	body, err := io.ReadAll(r.Body)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无法读取请求体: " + err.Error()})
		return
	}
	defer r.Body.Close()

	// 首先尝试解析整个请求
	var requestMap map[string]interface{}
	if err := json.Unmarshal(body, &requestMap); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无法解析请求: " + err.Error()})
		return
	}

	// 提取tag和type
	tag, tagExists := requestMap["tag"].(string)
	outboundType, typeExists := requestMap["type"].(string)

	if !tagExists || !typeExists {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "tag和type不能为空"})
		return
	}

	// 检查出站是否已存在
	if _, exists := s.outbound.Outbound(tag); exists {
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]string{"error": "出站已存在: " + tag})
		return
	}

	// 提取options
	var optionsRaw interface{}
	if options, hasOptions := requestMap["options"]; hasOptions {
		optionsRaw = options
	} else {
		// 如果没有options字段，将请求中除了tag和type外的所有字段作为options
		optionsMap := make(map[string]interface{})
		for key, value := range requestMap {
			if key != "tag" && key != "type" {
				optionsMap[key] = value
			}
		}
		optionsRaw = optionsMap
	}

	// 记录日志
	s.logger.Info("创建出站: ", outboundType, "[", tag, "]")

	// 获取出站注册表
	outboundRegistry := service.FromContext[adapter.OutboundRegistry](s.ctx)
	if outboundRegistry == nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "出站注册服务不可用"})
		return
	}

	// 创建出站配置对象
	optionsObj, exists := outboundRegistry.CreateOptions(outboundType)
	if !exists {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "不支持的出站类型: " + outboundType})
		return
	}

	// 将原始选项转换为正确的结构体
	optionsJson, err := json.Marshal(optionsRaw)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无法序列化选项: " + err.Error()})
		return
	}

	err = json.Unmarshal(optionsJson, optionsObj)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "选项格式错误: " + err.Error()})
		return
	}

	// 创建出站
	err = s.outbound.Create(s.ctx, s.router, s.logger, tag, outboundType, optionsObj)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "创建出站失败: " + err.Error()})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "出站创建成功",
		"tag":     tag,
		"type":    outboundType,
	})
}

// 移除出站
func (s *Server) removeOutbound(w http.ResponseWriter, r *http.Request) {
	tag := chi.URLParam(r, "tag")
	if tag == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "tag不能为空"})
		return
	}

	// 检查出站是否存在
	_, exists := s.outbound.Outbound(tag)
	if !exists {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": "出站不存在: " + tag})
		return
	}

	// 移除出站
	err := s.outbound.Remove(tag)
	if err != nil {
		if errors.Is(err, os.ErrInvalid) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{"error": "出站不存在: " + tag})
		} else {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, map[string]string{"error": "移除出站失败: " + err.Error()})
		}
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "出站移除成功",
		"tag":     tag,
	})
}

// 列出所有出站
func (s *Server) listOutbounds(w http.ResponseWriter, r *http.Request) {
	outbounds := s.outbound.Outbounds()
	var result []map[string]string

	for _, outbound := range outbounds {
		result = append(result, map[string]string{
			"tag":  outbound.Tag(),
			"type": outbound.Type(),
		})
	}

	render.JSON(w, r, map[string]interface{}{
		"success":   true,
		"outbounds": result,
	})
}

// 认证中间件
func authentication(serverSecret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if serverSecret == "" {
				next.ServeHTTP(w, r)
				return
			}

			if secret := r.Header.Get("Authorization"); secret != serverSecret {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{"error": "未经授权的访问"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// 创建路由规则
func (s *Server) createRouteRule(w http.ResponseWriter, r *http.Request) {
	// 从请求体中读取原始JSON数据
	body, err := io.ReadAll(r.Body)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无法读取请求体: " + err.Error()})
		return
	}
	defer r.Body.Close()

	// 首先尝试解析整个请求
	var requestMap map[string]interface{}
	if err := json.Unmarshal(body, &requestMap); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无法解析请求: " + err.Error()})
		return
	}

	// 检查outbound字段，这是必需的
	outboundRaw, hasOutbound := requestMap["outbound"]
	if !hasOutbound {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "outbound字段是必需的"})
		return
	}

	// 确保outbound是字符串
	outbound, ok := outboundRaw.(string)
	if !ok {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "outbound必须是字符串"})
		return
	}

	// 验证outbound标签存在
	if _, exists := s.outbound.Outbound(outbound); !exists {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "指定的出站不存在: " + outbound})
		return
	}

	// 处理inbounds字段，如果未提供则使用所有入站
	var inbounds []string
	inboundsRaw, hasInbounds := requestMap["inbounds"]

	if hasInbounds {
		// 确保inbounds是字符串数组
		switch v := inboundsRaw.(type) {
		case []interface{}:
			for _, item := range v {
				if strItem, ok := item.(string); ok {
					inbounds = append(inbounds, strItem)
				} else {
					render.Status(r, http.StatusBadRequest)
					render.JSON(w, r, map[string]string{"error": "inbounds必须是字符串数组"})
					return
				}
			}
		default:
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, map[string]string{"error": "inbounds必须是数组"})
			return
		}

		// 验证所有inbound标签都存在
		for _, inbound := range inbounds {
			if _, exists := s.inbound.Get(inbound); !exists {
				render.Status(r, http.StatusBadRequest)
				render.JSON(w, r, map[string]string{"error": "指定的入站不存在: " + inbound})
				return
			}
		}
	} else {
		// 如果没有提供inbounds，使用所有现有的入站
		for _, inb := range s.inbound.Inbounds() {
			inbounds = append(inbounds, inb.Tag())
		}

		// 如果没有任何入站，返回错误
		if len(inbounds) == 0 {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, map[string]string{"error": "系统中没有可用的入站，请先创建入站或在请求中指定inbounds"})
			return
		}
	}

	// 记录日志
	s.logger.Info("添加路由规则: 从 ", strings.Join(inbounds, ", "), " 到 ", outbound)

	// 准备rule对象
	rule := option.Rule{
		Type: "", // 默认为 "default" 类型规则
		DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{
				Inbound: inbounds,
			},
			RuleAction: option.RuleAction{
				Action: "route", // 设置为route动作
				RouteOptions: option.RouteActionOptions{
					Outbound: outbound,
				},
			},
		},
	}

	// 从请求中提取其他规则选项
	if processNameRaw, ok := requestMap["process_name"]; ok {
		if processNames, ok := processNameRaw.([]interface{}); ok {
			var processNameList []string
			for _, pn := range processNames {
				if pnStr, ok := pn.(string); ok {
					processNameList = append(processNameList, pnStr)
				}
			}
			if len(processNameList) > 0 {
				rule.DefaultOptions.RawDefaultRule.ProcessName = processNameList
			}
		}
	}

	// 添加对process_pid的处理
	if processPIDRaw, ok := requestMap["process_pid"]; ok {
		if processPIDs, ok := processPIDRaw.([]interface{}); ok {
			var processPIDList []uint32
			for _, pid := range processPIDs {
				if pidFloat, ok := pid.(float64); ok {
					processPIDList = append(processPIDList, uint32(pidFloat))
				} else if pidNumber, ok := pid.(json.Number); ok {
					if pidInt64, err := pidNumber.Int64(); err == nil {
						processPIDList = append(processPIDList, uint32(pidInt64))
					}
				}
			}
			if len(processPIDList) > 0 {
				rule.DefaultOptions.RawDefaultRule.ProcessPID = processPIDList
			}
		}
	}

	// 创建适配器Rule对象并添加到路由系统
	adapterRule, err := R.NewRule(s.ctx, s.logger, rule, true)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "创建规则失败: " + err.Error()})
		return
	}

	// 使用Router的AddRule方法添加规则
	ruleIndex := s.router.AddRule(adapterRule)

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]interface{}{
		"success":  true,
		"message":  "路由规则添加成功",
		"inbounds": inbounds,
		"outbound": outbound,
		"index":    ruleIndex,
	})
}

// 添加removeRouteRule方法
func (s *Server) removeRouteRule(w http.ResponseWriter, r *http.Request) {
	indexStr := chi.URLParam(r, "index")
	if indexStr == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "索引不能为空"})
		return
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "索引必须是有效的整数"})
		return
	}

	// 使用Router的RemoveRule方法移除规则
	err = s.router.RemoveRule(index)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "移除规则失败: " + err.Error()})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "路由规则已删除",
		"index":   index,
	})
}

// 添加listRouteRules方法
func (s *Server) listRouteRules(w http.ResponseWriter, r *http.Request) {
	// 获取实际路由规则列表
	rawRules := s.router.Rules()

	var rules []map[string]interface{}
	for i, rule := range rawRules {
		ruleMap := map[string]interface{}{
			"index":    i,
			"type":     rule.Type(),
			"outbound": rule.Action().String(),
			"desc":     rule.String(),
		}
		rules = append(rules, ruleMap)
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"rules":   rules,
	})
}
