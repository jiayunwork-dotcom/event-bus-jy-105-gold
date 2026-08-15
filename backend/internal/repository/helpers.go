package repository

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
