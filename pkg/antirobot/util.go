package antirobot

import (
	"strings"
)

// CollapseSpace 将连续空白（含换行、制表符）压缩为单个空格并 TrimSpace。
func CollapseSpace(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(s), " ")
}

// StripXMLTags 移除字符串中的 XML/HTML 标签。
func StripXMLTags(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
