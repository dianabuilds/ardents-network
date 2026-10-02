package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

func readBounded(filename string, maximum int64) ([]byte, error) {
	// Refuse a named pipe or device before Open, which can otherwise block.
	// Recheck the opened object as well; this is not filesystem custody.
	selected, err := os.Stat(filename)
	if err != nil || !selected.Mode().IsRegular() {
		return nil, errors.New("input unavailable")
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("input unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maximum+1))
	if err != nil || int64(len(raw)) > maximum {
		clear(raw)
		return nil, errors.New("input unavailable")
	}
	return raw, nil
}

func decodeFacts(raw []byte) (admission.Facts, error) {
	var f admission.Facts
	bad := errors.New("invalid facts")
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return f, bad
	}
	values := map[string]string{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return f, bad
		}
		name, ok := token.(string)
		if !ok {
			return f, bad
		}
		if _, exists := values[name]; exists {
			return f, bad
		}
		var value string
		if d.Decode(&value) != nil {
			return f, bad
		}
		values[name] = value
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return f, bad
	}
	if _, err = d.Token(); err != io.EOF {
		return f, bad
	}
	if len(values) != 10 {
		return f, bad
	}
	for name, target := range map[string]*[32]byte{"network": &f.Network, "issuer": &f.Issuer, "authority": &f.Authority, "holder": &f.Holder} {
		value, exists := values[name]
		if !exists || len(value) != 64 {
			return f, bad
		}
		b, err := hex.DecodeString(value)
		if err != nil {
			return f, bad
		}
		copy(target[:], b)
	}
	for name, target := range map[string]*time.Time{"duty_not_before": &f.DutyNotBefore, "duty_not_after": &f.DutyNotAfter, "now": &f.Now} {
		value, exists := values[name]
		if !exists {
			return f, bad
		}
		*target, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return f, bad
		}
	}
	for name, bits := range map[string]int{"duty": 64, "class": 8, "count": 32} {
		value, exists := values[name]
		if !exists {
			return f, bad
		}
		n, err := strconv.ParseUint(value, 10, bits)
		if err != nil || strconv.FormatUint(n, 10) != value {
			return f, bad
		}
		switch name {
		case "duty":
			f.Duty = n
		case "class":
			f.Class = uint8(n)
		case "count":
			f.Count = uint32(n)
		}
	}
	return f, nil
}
