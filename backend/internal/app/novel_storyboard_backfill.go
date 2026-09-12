package app

import (
	"strings"
)

// T6：对已有镜头做结构化字段回填（仅当字段为空时填充，避免覆盖人工修订）。
// storyboard 行与 shotID 序号对应（shot-001 → 镜1）。

func novelStoryboardRowForShot(rows map[int]novelStoryboardRow, shotID string) (novelStoryboardRow, bool) {
	digits := strings.TrimPrefix(shotID, "shot-")
	if digits == "" {
		return novelStoryboardRow{}, false
	}
	number := 0
	for _, r := range digits {
		if r < '0' || r > '9' {
			return novelStoryboardRow{}, false
		}
		number = number*10 + int(r-'0')
	}
	row, ok := rows[number]
	return row, ok
}
