package media

func yearFromDate(value string) string {
	if len(value) >= 4 {
		year := value[:4]
		if yearValuePattern.MatchString(year) {
			return year
		}
	}
	return ""
}
