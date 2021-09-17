package key

func IsCapiRelease(rel string) bool {
	return rel == "v20.0.0" || rel == "20.0.0"
}
