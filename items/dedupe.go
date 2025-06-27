package items

// DeDupeByUUID returns a new slice containing only the first occurrence of each
// item as identified by its UUID.
func DeDupeByUUID[T interface{ GetUUID() string }](in []T) []T {
	encountered := make(map[string]struct{})
	out := make([]T, 0, len(in))

	for _, v := range in {
		id := v.GetUUID()
		if _, ok := encountered[id]; ok {
			continue
		}
		encountered[id] = struct{}{}
		out = append(out, v)
	}

	return out
}
