package catalog

func cloneObjects(items map[string]Object) map[string]Object {
	cloned := make(map[string]Object, len(items))
	for id, item := range items {
		item.Body = cloneMap(item.Body)
		item.BlobRefs = append([]Ref(nil), item.BlobRefs...)
		cloned[id] = item
	}
	return cloned
}

func cloneManifests(items map[string]Manifest) map[string]Manifest {
	cloned := make(map[string]Manifest, len(items))
	for id, item := range items {
		item.Refs = append([]Ref(nil), item.Refs...)
		item.Metadata = cloneMap(item.Metadata)
		cloned[id] = item
	}
	return cloned
}

func cloneMap(items map[string]any) map[string]any {
	if items == nil {
		return nil
	}
	cloned := make(map[string]any, len(items))
	for key, item := range items {
		cloned[key] = cloneValue(item)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneValue(item)
		}
		return cloned
	case []string:
		return append([]string(nil), typed...)
	case []byte:
		return append([]byte(nil), typed...)
	default:
		return value
	}
}
