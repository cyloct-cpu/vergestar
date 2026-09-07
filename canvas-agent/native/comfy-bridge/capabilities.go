package main

import "strings"

func inferWorkflowCapabilities(workflow jsonMap) []string {
	capabilities := make([]string, 0, 1)
	contains := func(value string) bool {
		for _, item := range capabilities {
			if item == value {
				return true
			}
		}
		return false
	}
	add := func(value string) {
		if value != "" && !contains(value) {
			capabilities = append(capabilities, value)
		}
	}
	for _, rawNode := range iterWorkflowNodes(workflow) {
		node, ok := mapValue(rawNode)
		if !ok {
			continue
		}
		classType := strings.ToLower(firstNonEmpty(stringValue(node["class_type"]), stringValue(node["type"])))
		switch {
		case stringIn(classType, "saveimage", "saveimagewebsocket", "previewimage", "image_savetorch"):
			add("image")
		case strings.Contains(classType, "videocombine") || stringIn(classType, "savevideo", "savewebm", "saveanimatedmp4", "savevideowebm", "createvideo"):
			add("video")
		case strings.Contains(classType, "saveaudio") || strings.Contains(classType, "previewaudio") || stringIn(classType, "saveaudiomp3", "saveaudioopus", "saveaudioflac"):
			add("audio")
		}
	}
	if len(capabilities) == 0 {
		capabilities = inferCapabilitiesFromFields(workflow)
	}
	if len(capabilities) == 0 {
		capabilities = append(capabilities, "image")
	}
	return capabilities
}

func inferCapabilitiesFromFields(workflow jsonMap) []string {
	capabilities := make([]string, 0, 1)
	add := func(value string) {
		for _, item := range capabilities {
			if item == value {
				return
			}
		}
		capabilities = append(capabilities, value)
	}
	for _, rawNode := range iterWorkflowNodes(workflow) {
		node, ok := mapValue(rawNode)
		if !ok {
			continue
		}
		classType := strings.ToLower(firstNonEmpty(stringValue(node["class_type"]), stringValue(node["type"])))
		switch {
		case strings.Contains(classType, "loadimage"):
			add("image")
		case strings.Contains(classType, "loadvideo") || strings.Contains(classType, "createvideo"):
			add("video")
		case strings.Contains(classType, "loadaudio"):
			add("audio")
		}
	}
	return capabilities
}

// iterWorkflowNodes 从 API 或 UI 格式工作流中提取可执行节点。
func iterWorkflowNodes(workflow jsonMap) []any {
	if nodes := sliceValue(workflow["nodes"]); len(nodes) > 0 {
		return nodes
	}
	items := make([]any, 0, len(workflow))
	for _, raw := range workflow {
		items = append(items, raw)
	}
	return items
}
