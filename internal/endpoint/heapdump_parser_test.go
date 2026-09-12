package endpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
)

const heapDumpHeader = "go1.7 heap dump\n"

const (
	heapTagEOF             = 0
	heapTagObject          = 1
	heapTagOtherRoot       = 2
	heapTagType            = 3
	heapTagGoroutine       = 4
	heapTagStackFrame      = 5
	heapTagParams          = 6
	heapTagFinalizer       = 7
	heapTagItab            = 8
	heapTagOSThread        = 9
	heapTagMemStats        = 10
	heapTagQueuedFinalizer = 11
	heapTagData            = 12
	heapTagBSS             = 13
	heapTagDefer           = 14
	heapTagPanic           = 15
	heapTagMemProf         = 16
	heapTagAllocSample     = 17
)

type heapDumpReader struct {
	data []byte
	off  int
}

func (r *heapDumpReader) position() int { return r.off }

func (r *heapDumpReader) integer() (uint64, error) {
	var value uint64
	for shift := uint(0); ; shift += 7 {
		if shift >= 64 || r.off == len(r.data) {
			return 0, io.ErrUnexpectedEOF
		}
		part := r.data[r.off]
		r.off++
		if shift == 63 && part > 1 {
			return 0, errors.New("heap dump varint overflows uint64")
		}
		value |= uint64(part&0x7f) << shift
		if part < 0x80 {
			return value, nil
		}
	}
}

func (r *heapDumpReader) bytes() ([]byte, int, error) {
	length, err := r.integer()
	if err != nil {
		return nil, 0, err
	}
	if length > uint64(len(r.data)-r.off) {
		return nil, 0, io.ErrUnexpectedEOF
	}
	start := r.off
	r.off += int(length)
	return r.data[start:r.off], start, nil
}

func (r *heapDumpReader) string() (string, error) {
	value, _, err := r.bytes()
	return string(value), err
}

type heapDumpMatch struct {
	Record       string   `json:"record"`
	RecordOffset int      `json:"recordOffset"`
	MemoryOffset int      `json:"memoryOffset"`
	Address      string   `json:"address"`
	Object       string   `json:"object,omitempty"`
	Links        []string `json:"links"`
}

type heapDumpRoot struct {
	Description string `json:"description"`
	Address     string `json:"address"`
}

type heapDumpType struct {
	Address     string `json:"address"`
	Size        uint64 `json:"size"`
	Name        string `json:"name"`
	PointerData bool   `json:"pointerData"`
}

type heapDumpReport struct {
	Format               string          `json:"format"`
	TargetSHA256         string          `json:"targetSHA256"`
	ReceiptSHA256        string          `json:"receiptSHA256,omitempty"`
	TargetPresenceSHA256 string          `json:"targetPresenceSHA256,omitempty"`
	InputSHA256          string          `json:"inputSHA256"`
	ParsedBytes          int             `json:"parsedBytes"`
	Roots                []heapDumpRoot  `json:"roots"`
	Types                []heapDumpType  `json:"types"`
	Matches              []heapDumpMatch `json:"matches"`
	Limitation           string          `json:"limitation"`
}

type heapDumpMemory struct {
	record       string
	recordOffset int
	memoryOffset int
	address      uint64
	data         []byte
	fields       []uint64
}

func parseHeapDump(data, target []byte) (heapDumpReport, error) {
	if len(target) == 0 {
		return heapDumpReport{}, errors.New("empty heap dump target")
	}
	if !bytes.HasPrefix(data, []byte(heapDumpHeader)) {
		return heapDumpReport{}, errors.New("unrecognized heap dump header")
	}
	r := &heapDumpReader{data: data, off: len(heapDumpHeader)}
	var pointerSize int
	littleEndian := true
	var memories []heapDumpMemory
	var roots []heapDumpRoot
	var types []heapDumpType
	for {
		recordOffset := r.position()
		tag, err := r.integer()
		if err != nil {
			return heapDumpReport{}, fmt.Errorf("record at byte %d: %w", recordOffset, err)
		}
		if tag == heapTagEOF {
			if r.position() != len(data) {
				return heapDumpReport{}, fmt.Errorf("bytes after EOF at byte %d", r.position())
			}
			break
		}
		switch tag {
		case heapTagParams:
			bigEndian, err := r.integer()
			if err != nil || bigEndian > 1 {
				return heapDumpReport{}, fmt.Errorf("params at byte %d: invalid endianness", recordOffset)
			}
			littleEndian = bigEndian == 0
			size, err := r.integer()
			if err != nil || (size != 4 && size != 8) {
				return heapDumpReport{}, fmt.Errorf("params at byte %d: unsupported pointer size", recordOffset)
			}
			pointerSize = int(size)
			if err := discardIntegers(r, 2); err != nil {
				return heapDumpReport{}, err
			}
			if _, err := r.string(); err != nil {
				return heapDumpReport{}, err
			}
			if _, err := r.string(); err != nil {
				return heapDumpReport{}, err
			}
			if err := discardIntegers(r, 1); err != nil {
				return heapDumpReport{}, err
			}
		case heapTagObject:
			memory, err := readHeapMemory(r, "object", recordOffset)
			if err != nil {
				return heapDumpReport{}, err
			}
			fields, err := readHeapFields(r)
			if err != nil {
				return heapDumpReport{}, err
			}
			memory.fields = fields
			memories = append(memories, memory)
		case heapTagStackFrame:
			address, err := r.integer()
			if err != nil {
				return heapDumpReport{}, err
			}
			if err := discardIntegers(r, 2); err != nil {
				return heapDumpReport{}, err
			}
			memory, err := readHeapMemoryAt(r, "stack", recordOffset, address)
			if err != nil {
				return heapDumpReport{}, err
			}
			if err := discardIntegers(r, 3); err != nil {
				return heapDumpReport{}, err
			}
			if _, err := r.string(); err != nil {
				return heapDumpReport{}, err
			}
			fields, err := readHeapFields(r)
			if err != nil {
				return heapDumpReport{}, err
			}
			memory.fields = fields
			memories = append(memories, memory)
		case heapTagData, heapTagBSS:
			kind := "data"
			if tag == heapTagBSS {
				kind = "bss"
			}
			memory, err := readHeapMemory(r, kind, recordOffset)
			if err != nil {
				return heapDumpReport{}, err
			}
			fields, err := readHeapFields(r)
			if err != nil {
				return heapDumpReport{}, err
			}
			memory.fields = fields
			memories = append(memories, memory)
		case heapTagOtherRoot:
			description, err := r.string()
			if err != nil {
				return heapDumpReport{}, err
			}
			address, err := r.integer()
			if err != nil {
				return heapDumpReport{}, err
			}
			roots = append(roots, heapDumpRoot{Description: description, Address: fmt.Sprintf("0x%x", address)})
		case heapTagType:
			address, err := r.integer()
			if err != nil {
				return heapDumpReport{}, err
			}
			size, err := r.integer()
			if err != nil {
				return heapDumpReport{}, err
			}
			name, err := r.string()
			if err != nil {
				return heapDumpReport{}, err
			}
			pointerData, err := r.integer()
			if err != nil || pointerData > 1 {
				return heapDumpReport{}, fmt.Errorf("type at byte %d: invalid pointer data", recordOffset)
			}
			types = append(types, heapDumpType{Address: fmt.Sprintf("0x%x", address), Size: size, Name: name, PointerData: pointerData == 1})
		case heapTagGoroutine:
			if err := discardIntegers(r, 8); err != nil {
				return heapDumpReport{}, err
			}
			if _, err := r.string(); err != nil {
				return heapDumpReport{}, err
			}
			if err := discardIntegers(r, 4); err != nil {
				return heapDumpReport{}, err
			}
		case heapTagFinalizer, heapTagQueuedFinalizer:
			if err := discardIntegers(r, 5); err != nil {
				return heapDumpReport{}, err
			}
		case heapTagItab:
			if err := discardIntegers(r, 2); err != nil {
				return heapDumpReport{}, err
			}
		case heapTagOSThread:
			if err := discardIntegers(r, 3); err != nil {
				return heapDumpReport{}, err
			}
		case heapTagMemStats:
			if err := discardIntegers(r, 281); err != nil {
				return heapDumpReport{}, err
			}
		case heapTagDefer:
			if err := discardIntegers(r, 7); err != nil {
				return heapDumpReport{}, err
			}
		case heapTagPanic:
			if err := discardIntegers(r, 6); err != nil {
				return heapDumpReport{}, err
			}
		case heapTagMemProf:
			if err := discardIntegers(r, 2); err != nil {
				return heapDumpReport{}, err
			}
			frames, err := r.integer()
			if err != nil {
				return heapDumpReport{}, err
			}
			for range frames {
				if _, err := r.string(); err != nil {
					return heapDumpReport{}, err
				}
				if _, err := r.string(); err != nil {
					return heapDumpReport{}, err
				}
				if err := discardIntegers(r, 1); err != nil {
					return heapDumpReport{}, err
				}
			}
			if err := discardIntegers(r, 2); err != nil {
				return heapDumpReport{}, err
			}
		case heapTagAllocSample:
			if err := discardIntegers(r, 2); err != nil {
				return heapDumpReport{}, err
			}
		default:
			return heapDumpReport{}, fmt.Errorf("unknown heap dump tag %d at byte %d", tag, recordOffset)
		}
	}
	if pointerSize == 0 {
		return heapDumpReport{}, errors.New("heap dump has no params record")
	}
	inputHash := sha256.Sum256(data)
	targetHash := sha256.Sum256(target)
	report := heapDumpReport{
		Format:       "go1.7 heap dump",
		TargetSHA256: hex.EncodeToString(targetHash[:]),
		InputSHA256:  hex.EncodeToString(inputHash[:]),
		ParsedBytes:  len(data),
		Roots:        append([]heapDumpRoot{}, roots...),
		Types:        append([]heapDumpType{}, types...),
		Limitation:   "Go heap dumps contain object bytes and pointer-map offsets but do not attach a runtime type record to every object; report links are direct pointers only.",
	}
	for _, memory := range memories {
		for start := 0; ; {
			index := bytes.Index(memory.data[start:], target)
			if index < 0 {
				break
			}
			index += start
			report.Matches = append(report.Matches, heapDumpMatch{
				Record:       memory.record,
				RecordOffset: memory.recordOffset,
				MemoryOffset: memory.memoryOffset + index,
				Address:      fmt.Sprintf("0x%x", memory.address+uint64(index)),
				Object:       fmt.Sprintf("0x%x", memory.address),
				Links:        heapMemoryLinks(memory, pointerSize, littleEndian),
			})
			start = index + 1
		}
	}
	sort.Slice(report.Matches, func(i, j int) bool { return report.Matches[i].MemoryOffset < report.Matches[j].MemoryOffset })
	return report, nil
}

func discardIntegers(r *heapDumpReader, count int) error {
	for range count {
		if _, err := r.integer(); err != nil {
			return err
		}
	}
	return nil
}

func readHeapMemory(r *heapDumpReader, kind string, recordOffset int) (heapDumpMemory, error) {
	address, err := r.integer()
	if err != nil {
		return heapDumpMemory{}, err
	}
	data, offset, err := r.bytes()
	if err != nil {
		return heapDumpMemory{}, err
	}
	return heapDumpMemory{record: kind, recordOffset: recordOffset, memoryOffset: offset, address: address, data: data}, nil
}

func readHeapMemoryAt(r *heapDumpReader, kind string, recordOffset int, address uint64) (heapDumpMemory, error) {
	data, offset, err := r.bytes()
	if err != nil {
		return heapDumpMemory{}, err
	}
	return heapDumpMemory{record: kind, recordOffset: recordOffset, memoryOffset: offset, address: address, data: data}, nil
}

func readHeapFields(r *heapDumpReader) ([]uint64, error) {
	var fields []uint64
	for {
		kind, err := r.integer()
		if err != nil {
			return nil, err
		}
		if kind == 0 {
			return fields, nil
		}
		if kind != 1 {
			return nil, fmt.Errorf("unknown heap dump field kind %d", kind)
		}
		offset, err := r.integer()
		if err != nil {
			return nil, err
		}
		fields = append(fields, offset)
	}
}

func heapMemoryLinks(memory heapDumpMemory, pointerSize int, littleEndian bool) []string {
	links := make([]string, 0, len(memory.fields))
	for _, offset := range memory.fields {
		if offset > uint64(len(memory.data)-pointerSize) {
			continue
		}
		bytes := memory.data[offset : offset+uint64(pointerSize)]
		var value uint64
		if littleEndian {
			if pointerSize == 4 {
				value = uint64(binary.LittleEndian.Uint32(bytes))
			} else {
				value = binary.LittleEndian.Uint64(bytes)
			}
		} else if pointerSize == 4 {
			value = uint64(binary.BigEndian.Uint32(bytes))
		} else {
			value = binary.BigEndian.Uint64(bytes)
		}
		links = append(links, fmt.Sprintf("+0x%x->0x%x", offset, value))
	}
	return links
}

func TestParseHeapDumpRejectsTruncationAndUnknownTags(t *testing.T) {
	target := []byte{1, 2, 3, 4}
	valid := append([]byte(heapDumpHeader), 6, 0, 8, 0, 0, 5, 'a', 'm', 'd', '6', '4', 2, 'v', '1', 1, 0)
	report, err := parseHeapDump(valid, target)
	if err != nil {
		t.Fatal(err)
	}
	if report.ParsedBytes != len(valid) {
		t.Fatalf("parsed %d bytes, want %d", report.ParsedBytes, len(valid))
	}
	if _, err := parseHeapDump(valid[:len(valid)-1], target); err == nil {
		t.Fatal("truncated dump succeeded")
	}
	unknown := append(append([]byte{}, valid[:len(valid)-1]...), 99, 0)
	if _, err := parseHeapDump(unknown, target); err == nil || !strings.Contains(err.Error(), "unknown heap dump tag") {
		t.Fatalf("unknown tag error = %v", err)
	}
}

func TestParseHeapDumpMemProf(t *testing.T) {
	target := []byte{1, 2, 3, 4}
	dump := append([]byte(heapDumpHeader), 6, 0, 8, 0, 0, 5, 'a', 'm', 'd', '6', '4', 2, 'v', '1', 1)
	dump = append(dump, 16, 0, 0, 0, 0, 0, 0)
	if _, err := parseHeapDump(dump, target); err != nil {
		t.Fatal(err)
	}
}

func TestParseHeapDumpRecordsStackAddress(t *testing.T) {
	target := []byte{1, 2, 3, 4}
	dump := append([]byte(heapDumpHeader), 6, 0, 8, 0, 0, 5, 'a', 'm', 'd', '6', '4', 2, 'v', '1', 1)
	dump = append(dump, 5, 0x80, 0x02, 0, 0, 4, 1, 2, 3, 4, 0, 0, 0, 0, 0, 0)
	report, err := parseHeapDump(dump, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Matches) != 1 || report.Matches[0].Address != "0x100" {
		t.Fatalf("stack matches = %#v", report.Matches)
	}
}
