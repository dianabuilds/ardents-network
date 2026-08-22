package namespace

import "errors"

const (
	recordChunkSchema uint16 = 1
	maximumRecords           = 4096
	maximumChunks            = 64
	maximumChunkBytes        = 32 << 10
)

func encodeRecordChunks(records [][]byte) ([][]byte, error) {
	if len(records) == 0 || len(records) > maximumRecords {
		return nil, errors.New("naming materialization Record count is invalid")
	}
	chunks := [][]byte{}
	for index := 0; index < len(records); {
		chunk := appendUint16(nil, recordChunkSchema)
		chunk = appendUint16(chunk, 0)
		count := 0
		for index < len(records) {
			record := records[index]
			if len(record) == 0 || len(record)+4 > maximumChunkBytes-4 {
				return nil, errors.New("signed Name Record exceeds durable chunk bound")
			}
			if count > 0 && len(chunk)+4+len(record) > maximumChunkBytes {
				break
			}
			chunk = appendUint32(chunk, uint32(len(record)))
			chunk = append(chunk, record...)
			count++
			index++
		}
		chunk[2], chunk[3] = byte(count>>8), byte(count)
		chunks = append(chunks, chunk)
		if len(chunks) > maximumChunks {
			return nil, errors.New("naming materialization durable corpus exceeds its bound")
		}
	}
	return chunks, nil
}

func decodeRecordChunks(chunks [][]byte) ([][]byte, error) {
	if len(chunks) == 0 || len(chunks) > maximumChunks {
		return nil, errors.New("naming materialization durable chunks are invalid")
	}
	records := [][]byte{}
	for _, raw := range chunks {
		if len(raw) > maximumChunkBytes {
			return nil, errors.New("naming materialization durable chunk exceeds its bound")
		}
		cursor := byteCursor{raw: raw}
		schema, schemaErr := cursor.uint16()
		count, countErr := cursor.uint16()
		if schemaErr != nil || countErr != nil || schema != recordChunkSchema || count == 0 {
			return nil, errors.New("naming materialization durable chunk is malformed")
		}
		for range count {
			size, sizeErr := cursor.uint32()
			record, recordErr := cursor.bytes(int(size))
			if sizeErr != nil || recordErr != nil || size == 0 {
				return nil, errors.New("naming materialization durable Record is malformed")
			}
			records = append(records, append([]byte(nil), record...))
		}
		if !cursor.done() || len(records) > maximumRecords {
			return nil, errors.New("naming materialization durable chunk is non-canonical")
		}
	}
	canonical, err := encodeRecordChunks(records)
	if err != nil || !sameInputs(canonical, chunks) {
		return nil, errors.New("naming materialization durable corpus is non-canonical")
	}
	return records, nil
}
