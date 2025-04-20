package dynamicapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	R "github.com/sagernet/sing-box/route/rule"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

var _ adapter.DynamicManager = (*Server)(nil)

type Server struct {
	ctx              context.Context
	router           adapter.Router
	inbound          adapter.InboundManager
	outbound         adapter.OutboundManager
	logger           log.ContextLogger
	logFactory       log.Factory
	httpServer       *http.Server
	listenAddress    string
	secret           string
	enableConfigSave bool
	configSavePath   string
	dynamicConfig    *DynamicConfig
	inboundConfigs   map[string]map[string]interface{}
	outboundConfigs  map[string]map[string]interface{}
	ruleConfigs      []map[string]interface{}
	initialInbounds  map[string]bool
	initialOutbounds map[string]bool
}

// DynamicConfig 结构用于保存动态配置
type DynamicConfig struct {
	Inbounds  []map[string]interface{} `json:"inbounds,omitempty"`
	Outbounds []map[string]interface{} `json:"outbounds,omitempty"`
	Rules     []map[string]interface{} `json:"rules,omitempty"`
}

// TestOutboundRequest 结构用于测试出站请求
type TestOutboundRequest struct {
	Tag     string `json:"tag"`
	TestURL string `json:"test_url"`
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
		enableConfigSave: options.EnableConfigSave,
		configSavePath:   options.ConfigSavePath,
		inboundConfigs:   make(map[string]map[string]interface{}),
		outboundConfigs:  make(map[string]map[string]interface{}),
		ruleConfigs:      make([]map[string]interface{}, 0),
		initialInbounds:  make(map[string]bool),
		initialOutbounds: make(map[string]bool),
	}

	// 记录初始配置
	for _, inbound := range inboundManager.Inbounds() {
		s.initialInbounds[inbound.Tag()] = true
	}
	for _, outbound := range outboundManager.Outbounds() {
		s.initialOutbounds[outbound.Tag()] = true
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
			r.Post("/test", s.handleTestOutbound)
		})

		// 路由规则API
		r.Route("/route", func(r chi.Router) {
			r.Post("/rule", s.createRouteRule)
			r.Delete("/rule/{index}", s.removeRouteRule)
			r.Get("/rules", s.listRouteRules)
		})

		// 配置管理API
		r.Route("/config", func(r chi.Router) {
			r.Post("/save", s.handleSaveConfig)
			r.Post("/reload", s.handleReloadConfig)
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

	tag, ok := requestMap["tag"].(string)
	if !ok {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "tag不能为空"})
		return
	}

	// 保存原始配置
	s.inboundConfigs[tag] = requestMap

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
	s.logger.Info("创建入站: ", requestMap["type"], "[", tag, "]")

	// 获取入站注册表
	inboundRegistry := service.FromContext[adapter.InboundRegistry](s.ctx)
	if inboundRegistry == nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "入站注册服务不可用"})
		return
	}

	// 创建入站配置对象
	optionsObj, exists := inboundRegistry.CreateOptions(requestMap["type"].(string))
	if !exists {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "不支持的入站类型: " + requestMap["type"].(string)})
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
	err = s.inbound.Create(s.ctx, s.router, s.logger, tag, requestMap["type"].(string), optionsObj)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "创建入站失败: " + err.Error()})
		return
	}

	// 在成功创建入站后保存配置
	if s.enableConfigSave {
		if err := s.saveConfig(); err != nil {
			s.logger.Warn("保存配置失败:", err)
		}
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "入站创建成功",
		"tag":     tag,
		"type":    requestMap["type"],
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

	// 删除配置
	delete(s.inboundConfigs, tag)

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

	// 保存配置到文件
	if s.enableConfigSave {
		if err := s.saveConfig(); err != nil {
			s.logger.Warn("保存配置失败:", err)
		}
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

	tag, ok := requestMap["tag"].(string)
	if !ok {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "tag不能为空"})
		return
	}

	// 保存原始配置
	s.outboundConfigs[tag] = requestMap

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
	s.logger.Info("创建出站: ", requestMap["type"], "[", tag, "]")

	// 获取出站注册表
	outboundRegistry := service.FromContext[adapter.OutboundRegistry](s.ctx)
	if outboundRegistry == nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "出站注册服务不可用"})
		return
	}

	// 创建出站配置对象
	optionsObj, exists := outboundRegistry.CreateOptions(requestMap["type"].(string))
	if !exists {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "不支持的出站类型: " + requestMap["type"].(string)})
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
	err = s.outbound.Create(s.ctx, s.router, s.logger, tag, requestMap["type"].(string), optionsObj)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "创建出站失败: " + err.Error()})
		return
	}

	// 在成功创建出站后保存配置
	if s.enableConfigSave {
		if err := s.saveConfig(); err != nil {
			s.logger.Warn("保存配置失败:", err)
		}
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "出站创建成功",
		"tag":     tag,
		"type":    requestMap["type"],
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

	// 删除配置
	delete(s.outboundConfigs, tag)

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

	// 保存配置到文件
	if s.enableConfigSave {
		if err := s.saveConfig(); err != nil {
			s.logger.Warn("保存配置失败:", err)
		}
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

	// 保存原始配置
	s.ruleConfigs = append(s.ruleConfigs, requestMap)

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

	// 在成功创建规则后保存配置
	if s.enableConfigSave {
		if err := s.saveConfig(); err != nil {
			s.logger.Warn("保存配置失败:", err)
		}
	}

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

	// 删除配置
	if index >= 0 && index < len(s.ruleConfigs) {
		s.ruleConfigs = append(s.ruleConfigs[:index], s.ruleConfigs[index+1:]...)
	}

	// 使用Router的RemoveRule方法移除规则
	err = s.router.RemoveRule(index)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "移除规则失败: " + err.Error()})
		return
	}

	// 保存配置到文件
	if s.enableConfigSave {
		if err := s.saveConfig(); err != nil {
			s.logger.Warn("保存配置失败:", err)
		}
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

// saveConfig 保存当前的动态配置到文件
func (s *Server) saveConfig() error {
	if !s.enableConfigSave || s.configSavePath == "" {
		return nil
	}

	// 构建配置
	config := &DynamicConfig{
		Inbounds:  make([]map[string]interface{}, 0),
		Outbounds: make([]map[string]interface{}, 0),
		Rules:     make([]map[string]interface{}, 0),
	}

	// 获取所有入站配置
	for _, inbound := range s.inbound.Inbounds() {
		// 跳过初始配置中的入站
		if s.initialInbounds[inbound.Tag()] {
			s.logger.Debug("跳过初始入站:", inbound.Tag())
			continue
		}

		// 跳过系统入站（如tun）
		if inbound.Type() == "tun" {
			s.logger.Debug("跳过系统入站:", inbound.Tag(), "(", inbound.Type(), ")")
			continue
		}

		// 获取保存的配置
		if savedConfig, exists := s.inboundConfigs[inbound.Tag()]; exists {
			config.Inbounds = append(config.Inbounds, savedConfig)
		}
	}

	// 获取所有出站配置
	for _, outbound := range s.outbound.Outbounds() {
		// 跳过初始配置中的出站
		if s.initialOutbounds[outbound.Tag()] {
			s.logger.Debug("跳过初始出站:", outbound.Tag())
			continue
		}

		// 获取保存的配置
		if savedConfig, exists := s.outboundConfigs[outbound.Tag()]; exists {
			config.Outbounds = append(config.Outbounds, savedConfig)
		}
	}

	// 获取所有规则配置
	config.Rules = s.ruleConfigs

	// 将配置写入文件，使用缩进格式化
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		s.logger.Error("序列化最终配置失败:", err)
		return err
	}

	s.logger.Debug("保存的完整配置:", string(configData))
	return os.WriteFile(s.configSavePath, configData, 0644)
}

// 添加配置重载相关的辅助方法
func (s *Server) createInboundFromConfig(config map[string]interface{}) error {
	tag, ok := config["tag"].(string)
	if !ok {
		return errors.New("入站配置缺少tag字段")
	}

	inboundType, ok := config["type"].(string)
	if !ok {
		return errors.New("入站配置缺少type字段")
	}

	// 获取入站注册表
	inboundRegistry := service.FromContext[adapter.InboundRegistry](s.ctx)
	if inboundRegistry == nil {
		return errors.New("入站注册服务不可用")
	}

	// 创建入站配置对象
	optionsObj, exists := inboundRegistry.CreateOptions(inboundType)
	if !exists {
		return errors.New("不支持的入站类型: " + inboundType)
	}

	// 将配置转换为正确的结构体
	optionsJson, err := json.Marshal(config)
	if err != nil {
		return err
	}

	err = json.Unmarshal(optionsJson, optionsObj)
	if err != nil {
		return err
	}

	// 创建入站
	return s.inbound.Create(s.ctx, s.router, s.logger, tag, inboundType, optionsObj)
}

func (s *Server) createOutboundFromConfig(config map[string]interface{}) error {
	tag, ok := config["tag"].(string)
	if !ok {
		return errors.New("出站配置缺少tag字段")
	}

	outboundType, ok := config["type"].(string)
	if !ok {
		return errors.New("出站配置缺少type字段")
	}

	// 获取出站注册表
	outboundRegistry := service.FromContext[adapter.OutboundRegistry](s.ctx)
	if outboundRegistry == nil {
		return errors.New("出站注册服务不可用")
	}

	// 创建出站配置对象
	optionsObj, exists := outboundRegistry.CreateOptions(outboundType)
	if !exists {
		return errors.New("不支持的出站类型: " + outboundType)
	}

	// 将配置转换为正确的结构体
	optionsJson, err := json.Marshal(config)
	if err != nil {
		return err
	}

	err = json.Unmarshal(optionsJson, optionsObj)
	if err != nil {
		return err
	}

	// 创建出站
	return s.outbound.Create(s.ctx, s.router, s.logger, tag, outboundType, optionsObj)
}

func (s *Server) createRuleFromConfig(config map[string]interface{}) error {
	ruleJson, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var rule option.Rule
	if err := json.Unmarshal(ruleJson, &rule); err != nil {
		return err
	}

	adapterRule, err := R.NewRule(s.ctx, s.logger, rule, true)
	if err != nil {
		return err
	}

	s.router.AddRule(adapterRule)
	return nil
}

// loadConfig 从文件加载配置
func (s *Server) loadConfig() error {
	if !s.enableConfigSave || s.configSavePath == "" {
		return nil
	}

	// 读取配置文件
	configData, err := os.ReadFile(s.configSavePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var config DynamicConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return err
	}

	// 获取当前所有入站
	currentInbounds := s.inbound.Inbounds()
	systemInbounds := make(map[string]bool)

	// 标记系统入站（如tun）
	for _, inbound := range currentInbounds {
		if inbound.Type() == "tun" {
			systemInbounds[inbound.Tag()] = true
			continue
		}
	}

	// 清除现有配置，但保留系统入站
	for _, inbound := range currentInbounds {
		if !systemInbounds[inbound.Tag()] {
			s.inbound.Remove(inbound.Tag())
		}
	}

	// 清除所有出站
	for _, outbound := range s.outbound.Outbounds() {
		s.outbound.Remove(outbound.Tag())
	}

	// 清除所有规则
	rules := s.router.Rules()
	for i := len(rules) - 1; i >= 0; i-- {
		s.router.RemoveRule(i)
	}

	// 重新加载入站配置，跳过与系统入站冲突的配置
	for _, inboundConfig := range config.Inbounds {
		tag, ok := inboundConfig["tag"].(string)
		if !ok {
			s.logger.Warn("入站配置缺少tag字段")
			continue
		}

		// 如果是系统入站，跳过
		if systemInbounds[tag] {
			s.logger.Info("跳过系统入站配置:", tag)
			continue
		}

		if err := s.createInboundFromConfig(inboundConfig); err != nil {
			s.logger.Warn("加载入站配置失败:", err)
		}
	}

	// 重新加载出站配置
	for _, outboundConfig := range config.Outbounds {
		if err := s.createOutboundFromConfig(outboundConfig); err != nil {
			s.logger.Warn("加载出站配置失败:", err)
		}
	}

	// 重新加载规则配置
	for _, ruleConfig := range config.Rules {
		if err := s.createRuleFromConfig(ruleConfig); err != nil {
			s.logger.Warn("加载规则配置失败:", err)
		}
	}

	return nil
}

// handleTestOutbound 处理出站测试请求
func (s *Server) handleTestOutbound(w http.ResponseWriter, r *http.Request) {
	var request TestOutboundRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无法解析请求: " + err.Error()})
		return
	}

	// 获取出站
	outbound, exists := s.outbound.Outbound(request.Tag)
	if !exists || outbound == nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": "出站不存在: " + request.Tag})
		return
	}

	// 如果没有提供测试URL，使用默认的
	if request.TestURL == "" {
		request.TestURL = "http://www.gstatic.com/generate_204"
	}

	// 验证URL格式
	if _, err := url.Parse(request.TestURL); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "无效的URL: " + err.Error()})
		return
	}

	// 创建HTTP客户端
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 解析地址
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
				switch network {
				case "tcp", "tcp4", "tcp6":
					port = "80"
				default:
					return nil, E.New("unsupported network: ", network)
				}
			}

			// 创建 Socksaddr
			portNum, err := strconv.Atoi(port)
			if err != nil {
				return nil, err
			}

			destination := metadata.Socksaddr{
				Fqdn: host,
				Port: uint16(portNum),
			}

			// 如果是IP地址，则解析它
			if ip := net.ParseIP(host); ip != nil {
				if addr, err := netip.ParseAddr(host); err == nil {
					destination.Addr = addr
					destination.Fqdn = ""
				} else {
					return nil, E.New("invalid IP address: ", host)
				}
			}

			return outbound.DialContext(ctx, network, destination)
		},
		// 禁用默认的TLS验证，因为某些代理可能使用自签名证书
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	// 记录开始时间
	startTime := time.Now()

	// 发送测试请求
	resp, err := client.Get(request.TestURL)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"status": "failed",
			"error":  err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// 计算延迟
	delay := time.Since(startTime)

	// 读取响应体（但限制大小）
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	// 返回测试结果
	render.JSON(w, r, map[string]interface{}{
		"status":      "success",
		"delay_ms":    delay.Milliseconds(),
		"status_code": resp.StatusCode,
		"body_length": len(body),
		"headers":     resp.Header,
	})
}

// 添加新的API路由处理函数
func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if err := s.saveConfig(); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "保存配置失败: " + err.Error()})
		return
	}
	render.JSON(w, r, map[string]string{"message": "配置已保存"})
}

func (s *Server) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	if err := s.loadConfig(); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "重载配置失败: " + err.Error()})
		return
	}
	render.JSON(w, r, map[string]string{"message": "配置已重载"})
}
