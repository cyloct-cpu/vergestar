package app

import (
	"strconv"
	"strings"
)

// T6：分镜 Markdown 解析——按镜号提取景别/机位/时长/对白/备注，填充 ShotRevision。
// 兼容两种 InkOS/gpt-5.x 常见格式：
//  A) 表格式：| 镜号 | 场景/画面 | 人物/物件 | 动作 | 景别/机位 | 对白/字幕 | 时长建议 | 备注 |
//  B) 列表式：**1. 镜号：01** 标题 + *   **画面**：… 字段 bullets

type novelStoryboardRow struct {
	Number          int
	Scene           string
	Characters      string
	Action          string
	ShotSize        string
	CameraAngle     string
	CameraMovement  string
	Dialogue        string
	DurationMs      int64
	ContinuityNotes string
}

// parseNovelStoryboardTable extracts the shot table rows keyed by shot number.
// Returns nil-safe empty slice when the table is absent.
// 列表式分镜解析（格式 B）：**N. 镜号：NN** + *   **字段**：值 bullets。
func parseNovelStoryboardList(storyboard string) map[int]novelStoryboardRow {
    rows := make(map[int]novelStoryboardRow)
    var current *novelStoryboardRow
    currentNumber := 0
    flush := func() {
        if current != nil && currentNumber > 0 {
            rows[currentNumber] = *current
        }
        current = nil
    }
    for _, raw := range strings.Split(storyboard, "\n") {
        line := strings.TrimSpace(raw)
        if line == "" {
            continue
        }
        // 新镜条目：**1. 镜号：01**（兼容 **镜 1** 等变体）
        if strings.HasPrefix(line, "**") {
            inner := strings.Trim(line, "*")
            inner = strings.TrimSpace(inner)
            if strings.Contains(inner, "镜号") {
                flush()
                number := 0
                digitStart := 0
                for digitStart < len(inner) && (inner[digitStart] < '0' || inner[digitStart] > '9') {
                    digitStart++
                }
                digitEnd := digitStart
                for digitEnd < len(inner) && inner[digitEnd] >= '0' && inner[digitEnd] <= '9' {
                    digitEnd++
                }
                if digitEnd > digitStart {
                    if n, err := strconv.Atoi(inner[digitStart:digitEnd]); err == nil && n > 0 {
                        number = n
                    }
                }
                if number <= 0 {
                    // 镜号无法解析的条目跳过，不进入 flush 缓存。
                    current = nil
                    continue
                }
                currentNumber = number
                current = &novelStoryboardRow{Number: number}
                // 标题行可能内嵌对白/时长等尾部字段，忽略——bullets 会覆盖。
                continue
            }
        }
        if current == nil {
            continue
        }
        // 字段 bullet：*   **画面**：值
        bullet := strings.TrimPrefix(line, "*")
        bullet = strings.TrimSpace(bullet)
        if !strings.HasPrefix(bullet, "**") {
            continue
        }
        sep := strings.Index(bullet[2:], "**")
        if sep < 0 {
            continue
        }
        fieldName := strings.TrimSpace(bullet[2 : 2+sep])
        value := strings.TrimSpace(bullet[2+sep+2:])
        value = strings.Trim(value, "：: ")
        switch {
        case strings.HasPrefix(fieldName, "画面"):
            current.Scene = value
        case strings.HasPrefix(fieldName, "人物"):
            current.Characters = value
        case strings.HasPrefix(fieldName, "动作"):
            current.Action = value
        case strings.HasPrefix(fieldName, "景别"), strings.HasPrefix(fieldName, "机位"):
            current.ShotSize = novelShotSizeFrom(value)
            current.CameraAngle = novelCameraAngleFrom(value)
        case strings.HasPrefix(fieldName, "对白"), strings.HasPrefix(fieldName, "字幕"):
            current.Dialogue = value
        case strings.Contains(fieldName, "时长"):
            current.DurationMs = novelDurationToMs(value)
        case strings.HasPrefix(fieldName, "备注"):
            current.ContinuityNotes = value
        }
    }
    flush()
    return rows
}

func parseNovelStoryboardTable(storyboard string) map[int]novelStoryboardRow {
	if listRows := parseNovelStoryboardList(storyboard); len(listRows) > 0 {
		return listRows
	}
	rows := make(map[int]novelStoryboardRow)
	if storyboard == "" {
		return rows
	}
	var headerCols []string
	for _, raw := range strings.Split(storyboard, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := splitNovelMarkdownRow(line)
		if len(cells) < 3 {
			continue
		}
		if headerCols == nil {
			if strings.Contains(line, "镜号") {
				headerCols = cells
			}
			continue
		}
		if looksLikeNovelTableDivider(line) {
			continue
		}
		colIndex := func(names ...string) int {
			for i, col := range headerCols {
				for _, name := range names {
					if strings.Contains(col, name) {
						return i
					}
				}
			}
			return -1
		}
		numberIdx := colIndex("镜号")
		if numberIdx < 0 || numberIdx >= len(cells) {
			continue
		}
		// 提取镜号单元格中的前导数字（"10" 不能被字符集 Trim 破坏）。
		cell := strings.TrimSpace(cells[numberIdx])
		digitEnd := 0
		for digitEnd < len(cell) && cell[digitEnd] >= '0' && cell[digitEnd] <= '9' {
			digitEnd++
		}
		number, err := strconv.Atoi(cell[:digitEnd])
		if err != nil || number <= 0 {
			continue
		}
		get := func(names ...string) string {
			idx := colIndex(names...)
			if idx < 0 || idx >= len(cells) {
				return ""
			}
			return strings.TrimSpace(cells[idx])
		}
		rows[number] = novelStoryboardRow{
			Number:          number,
			Scene:           get("场景", "画面"),
			Characters:      get("人物", "物件"),
			Action:          get("动作"),
			ShotSize:        get("景别", "机位"),
			CameraAngle:     get("景别", "机位"),
			Dialogue:        get("对白", "字幕"),
			ContinuityNotes: get("备注"),
			DurationMs:      novelDurationToMs(get("时长")),
		}
	}
	return rows
}

func splitNovelMarkdownRow(line string) []string {
	trimmed := strings.Trim(line, "|")
	parts := strings.Split(trimmed, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func looksLikeNovelTableDivider(line string) bool {
	compact := strings.ReplaceAll(strings.ReplaceAll(line, "|", ""), "-", "")
	return strings.TrimSpace(compact) == ""
}

// novelDurationToMs parses "3秒" / "4s" / "3000ms" style durations.
func novelDurationToMs(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
	if digits == "" {
		return 0
	}
	number, err := strconv.Atoi(digits)
	if err != nil || number <= 0 {
		return 0
	}
	if strings.Contains(value, "ms") {
		return int64(number)
	}
	return int64(number) * 1000
}

// novelShotSizeAndCamera splits "景别/机位" text into shot size and camera angle
// keywords following the domain vocabulary used by InkOS storyboards.
var novelShotSizeKeywords = []string{"大特写", "特写", "近景", "中景", "远景", "全景", "过肩", "手部特写", "大远景"}

var novelCameraKeywords = []string{"跟拍", "俯拍", "仰拍", "低角度", "过肩", "手持", "环绕", "推镜", "拉镜", "平视", "俯视"}

func novelShotSizeFrom(cell string) string {
	for _, keyword := range novelShotSizeKeywords {
		if strings.Contains(cell, keyword) {
			return keyword
		}
	}
	return strings.TrimSpace(cell)
}

func novelCameraAngleFrom(cell string) string {
	for _, keyword := range novelCameraKeywords {
		if strings.Contains(cell, keyword) {
			return keyword
		}
	}
	// 没有明确机位关键词时保留原始景别/机位描述。
	return strings.TrimSpace(cell)
}
