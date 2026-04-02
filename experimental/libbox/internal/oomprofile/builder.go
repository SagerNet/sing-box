//go:build darwin || linux || windows

package oomprofile

import (
	"fmt"
	"io"
	"runtime"
	"time"
)

const (
	tagProfile_SampleType        = 1
	tagProfile_Sample            = 2
	tagProfile_Mapping           = 3
	tagProfile_Location          = 4
	tagProfile_Function          = 5
	tagProfile_StringTable       = 6
	tagProfile_TimeNanos         = 9
	tagProfile_PeriodType        = 11
	tagProfile_Period            = 12
	tagProfile_DefaultSampleType = 14

	tagValueType_Type = 1
	tagValueType_Unit = 2

	tagSample_Location = 1
	tagSample_Value    = 2
	tagSample_Label    = 3

	tagLabel_Key = 1
	tagLabel_Str = 2
	tagLabel_Num = 3

	tagMapping_ID              = 1
	tagMapping_Start           = 2
	tagMapping_Limit           = 3
	tagMapping_Offset          = 4
	tagMapping_Filename        = 5
	tagMapping_BuildID         = 6
	tagMapping_HasFunctions    = 7
	tagMapping_HasFilenames    = 8
	tagMapping_HasLineNumbers  = 9
	tagMapping_HasInlineFrames = 10

	tagLocation_ID        = 1
	tagLocation_MappingID = 2
	tagLocation_Address   = 3
	tagLocation_Line      = 4

	tagLine_FunctionID = 1
	tagLine_Line       = 2

	tagFunction_ID         = 1
	tagFunction_Name       = 2
	tagFunction_SystemName = 3
	tagFunction_Filename   = 4
	tagFunction_StartLine  = 5
)

type memMap struct {
	start   uintptr
	end     uintptr
	offset  uint64
	file    string
	buildID string
	funcs   symbolizeFlag
	fake    bool
}

type symbolizeFlag uint8

const (
	lookupTried symbolizeFlag = 1 << iota
	lookupFailed
)

func newProfileBuilder(w io.Writer) *profileBuilder {
	builder := &profileBuilder{
		start:     time.Now(),
		w:         w,
		strings:   []string{""},
		stringMap: map[string]int{"": 0},
		locs:      map[uintptr]locInfo{},
		funcs:     map[string]int{},
	}
	builder.readMapping()
	return builder
}

func (b *profileBuilder) stringIndex(s string) int64 {
	id, ok := b.stringMap[s]
	if !ok {
		id = len(b.strings)
		b.strings = append(b.strings, s)
		b.stringMap[s] = id
	}
	return int64(id)
}

func (b *profileBuilder) flush() {
	const dataFlush = 4096
	if b.err != nil || b.pb.nest != 0 || len(b.pb.data) <= dataFlush {
		return
	}

	_, b.err = b.w.Write(b.pb.data)
	b.pb.data = b.pb.data[:0]
}

func (b *profileBuilder) pbValueType(tag int, typ string, unit string) {
	start := b.pb.startMessage()
	b.pb.int64(tagValueType_Type, b.stringIndex(typ))
	b.pb.int64(tagValueType_Unit, b.stringIndex(unit))
	b.pb.endMessage(tag, start)
}

func (b *profileBuilder) pbSample(values []int64, locs []uint64, labels func()) {
	start := b.pb.startMessage()
	b.pb.int64s(tagSample_Value, values)
	b.pb.uint64s(tagSample_Location, locs)
	if labels != nil {
		labels()
	}
	b.pb.endMessage(tagProfile_Sample, start)
	b.flush()
}

func (b *profileBuilder) pbLabel(tag int, key string, str string, num int64) {
	start := b.pb.startMessage()
	b.pb.int64Opt(tagLabel_Key, b.stringIndex(key))
	b.pb.int64Opt(tagLabel_Str, b.stringIndex(str))
	b.pb.int64Opt(tagLabel_Num, num)
	b.pb.endMessage(tag, start)
}

func (b *profileBuilder) pbLine(tag int, funcID uint64, line int64) {
	start := b.pb.startMessage()
	b.pb.uint64Opt(tagLine_FunctionID, funcID)
	b.pb.int64Opt(tagLine_Line, line)
	b.pb.endMessage(tag, start)
}

func (b *profileBuilder) pbMapping(tag int, id uint64, base uint64, limit uint64, offset uint64, file string, buildID string, hasFuncs bool) {
	start := b.pb.startMessage()
	b.pb.uint64Opt(tagMapping_ID, id)
	b.pb.uint64Opt(tagMapping_Start, base)
	b.pb.uint64Opt(tagMapping_Limit, limit)
	b.pb.uint64Opt(tagMapping_Offset, offset)
	b.pb.int64Opt(tagMapping_Filename, b.stringIndex(file))
	b.pb.int64Opt(tagMapping_BuildID, b.stringIndex(buildID))
	if hasFuncs {
		b.pb.bool(tagMapping_HasFunctions, true)
	}
	b.pb.endMessage(tag, start)
}

func (b *profileBuilder) build() error {
	if b.err != nil {
		return b.err
	}

	b.pb.int64Opt(tagProfile_TimeNanos, b.start.UnixNano())
	for i, mapping := range b.mem {
		hasFunctions := mapping.funcs == lookupTried
		b.pbMapping(tagProfile_Mapping, uint64(i+1), uint64(mapping.start), uint64(mapping.end), mapping.offset, mapping.file, mapping.buildID, hasFunctions)
	}
	b.pb.strings(tagProfile_StringTable, b.strings)
	if b.err != nil {
		return b.err
	}
	_, err := b.w.Write(b.pb.data)
	return err
}

func allFrames(addr uintptr) ([]runtime.Frame, symbolizeFlag) {
	frames := runtime.CallersFrames([]uintptr{addr})
	frame, more := frames.Next()
	if frame.Function == "runtime.goexit" {
		return nil, 0
	}

	result := lookupTried
	if frame.PC == 0 || frame.Function == "" || frame.File == "" || frame.Line == 0 {
		result |= lookupFailed
	}
	if frame.PC == 0 {
		frame.PC = addr - 1
	}

	ret := []runtime.Frame{frame}
	for frame.Function != "runtime.goexit" && more {
		frame, more = frames.Next()
		ret = append(ret, frame)
	}
	return ret, result
}

type locInfo struct {
	id uint64

	pcs []uintptr

	firstPCFrames          []runtime.Frame
	firstPCSymbolizeResult symbolizeFlag
}

func (b *profileBuilder) appendLocsForStack(locs []uint64, stk []uintptr) []uint64 {
	b.deck.reset()
	origStk := stk
	stk = runtimeExpandFinalInlineFrame(stk)

	for len(stk) > 0 {
		addr := stk[0]
		if loc, ok := b.locs[addr]; ok {
			if len(b.deck.pcs) > 0 {
				if b.deck.tryAdd(addr, loc.firstPCFrames, loc.firstPCSymbolizeResult) {
					stk = stk[1:]
					continue
				}
			}
			if id := b.emitLocation(); id > 0 {
				locs = append(locs, id)
			}
			locs = append(locs, loc.id)
			if len(loc.pcs) > len(stk) {
				panic(fmt.Sprintf("stack too short to match cached location; stk = %#x, loc.pcs = %#x, original stk = %#x", stk, loc.pcs, origStk))
			}
			stk = stk[len(loc.pcs):]
			continue
		}

		frames, symbolizeResult := allFrames(addr)
		if len(frames) == 0 {
			if id := b.emitLocation(); id > 0 {
				locs = append(locs, id)
			}
			stk = stk[1:]
			continue
		}

		if b.deck.tryAdd(addr, frames, symbolizeResult) {
			stk = stk[1:]
			continue
		}
		if id := b.emitLocation(); id > 0 {
			locs = append(locs, id)
		}

		if loc, ok := b.locs[addr]; ok {
			locs = append(locs, loc.id)
			stk = stk[len(loc.pcs):]
		} else {
			b.deck.tryAdd(addr, frames, symbolizeResult)
			stk = stk[1:]
		}
	}
	if id := b.emitLocation(); id > 0 {
		locs = append(locs, id)
	}
	return locs
}

type pcDeck struct {
	pcs             []uintptr
	frames          []runtime.Frame
	symbolizeResult symbolizeFlag

	firstPCFrames          int
	firstPCSymbolizeResult symbolizeFlag
}

func (d *pcDeck) reset() {
	d.pcs = d.pcs[:0]
	d.frames = d.frames[:0]
	d.symbolizeResult = 0
	d.firstPCFrames = 0
	d.firstPCSymbolizeResult = 0
}

func (d *pcDeck) tryAdd(pc uintptr, frames []runtime.Frame, symbolizeResult symbolizeFlag) bool {
	if existing := len(d.frames); existing > 0 {
		newFrame := frames[0]
		last := d.frames[existing-1]
		if last.Func != nil {
			return false
		}
		if last.Entry == 0 || newFrame.Entry == 0 {
			return false
		}
		if last.Entry != newFrame.Entry {
			return false
		}
		if runtimeFrameSymbolName(&last) == runtimeFrameSymbolName(&newFrame) {
			return false
		}
	}

	d.pcs = append(d.pcs, pc)
	d.frames = append(d.frames, frames...)
	d.symbolizeResult |= symbolizeResult
	if len(d.pcs) == 1 {
		d.firstPCFrames = len(d.frames)
		d.firstPCSymbolizeResult = symbolizeResult
	}
	return true
}

func (b *profileBuilder) emitLocation() uint64 {
	if len(b.deck.pcs) == 0 {
		return 0
	}
	defer b.deck.reset()

	addr := b.deck.pcs[0]
	firstFrame := b.deck.frames[0]

	type newFunc struct {
		id        uint64
		name      string
		file      string
		startLine int64
	}

	newFuncs := make([]newFunc, 0, 8)
	id := uint64(len(b.locs)) + 1
	b.locs[addr] = locInfo{
		id:                     id,
		pcs:                    append([]uintptr{}, b.deck.pcs...),
		firstPCFrames:          append([]runtime.Frame{}, b.deck.frames[:b.deck.firstPCFrames]...),
		firstPCSymbolizeResult: b.deck.firstPCSymbolizeResult,
	}

	start := b.pb.startMessage()
	b.pb.uint64Opt(tagLocation_ID, id)
	b.pb.uint64Opt(tagLocation_Address, uint64(firstFrame.PC))
	for _, frame := range b.deck.frames {
		funcName := runtimeFrameSymbolName(&frame)
		funcID := uint64(b.funcs[funcName])
		if funcID == 0 {
			funcID = uint64(len(b.funcs)) + 1
			b.funcs[funcName] = int(funcID)
			newFuncs = append(newFuncs, newFunc{
				id:        funcID,
				name:      funcName,
				file:      frame.File,
				startLine: int64(runtimeFrameStartLine(&frame)),
			})
		}
		b.pbLine(tagLocation_Line, funcID, int64(frame.Line))
	}
	for i := range b.mem {
		if (b.mem[i].start <= addr && addr < b.mem[i].end) || b.mem[i].fake {
			b.pb.uint64Opt(tagLocation_MappingID, uint64(i+1))
			mapping := b.mem[i]
			mapping.funcs |= b.deck.symbolizeResult
			b.mem[i] = mapping
			break
		}
	}
	b.pb.endMessage(tagProfile_Location, start)

	for _, fn := range newFuncs {
		start := b.pb.startMessage()
		b.pb.uint64Opt(tagFunction_ID, fn.id)
		b.pb.int64Opt(tagFunction_Name, b.stringIndex(fn.name))
		b.pb.int64Opt(tagFunction_SystemName, b.stringIndex(fn.name))
		b.pb.int64Opt(tagFunction_Filename, b.stringIndex(fn.file))
		b.pb.int64Opt(tagFunction_StartLine, fn.startLine)
		b.pb.endMessage(tagProfile_Function, start)
	}

	b.flush()
	return id
}

func (b *profileBuilder) addMapping(lo uint64, hi uint64, offset uint64, file string, buildID string) {
	b.addMappingEntry(lo, hi, offset, file, buildID, false)
}

func (b *profileBuilder) addMappingEntry(lo uint64, hi uint64, offset uint64, file string, buildID string, fake bool) {
	b.mem = append(b.mem, memMap{
		start:   uintptr(lo),
		end:     uintptr(hi),
		offset:  offset,
		file:    file,
		buildID: buildID,
		fake:    fake,
	})
}
