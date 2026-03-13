package utils

func RecursiveMerge(dicts ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, d := range dicts {
		if d == nil {
			continue
		}
		for k, v := range d {
			existing, exists := result[k]
			if exists {
				if existMap, ok := existing.(map[string]any); ok {
					if vMap, ok2 := v.(map[string]any); ok2 {
						result[k] = RecursiveMerge(existMap, vMap)
						continue
					}
				}
			}
			result[k] = v
		}
	}
	return result
}
