package main

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var numberPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

func sortWorkflowList(items []jsonMap) {
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(stringValue(items[i]["title"])) < strings.ToLower(stringValue(items[j]["title"]))
	})
}

func discoverWorkflowFields(workflow jsonMap, capability string) []any {
	fields := make([]any, 0)
	nodeIDs := sortedNodeIDs(workflow)
	promptNode, promptField := findWorkflowPromptTarget(workflow, nodeIDs)
	promptFields := promptTargetFields(workflow, nodeIDs)
	graph := workflowProducerGraph(workflow)
	branches := workflowImageBranches(workflow, nodeIDs, graph)
	imageOrder, videoOrder, audioOrder := 0, 0, 0
	for _, nodeID := range nodeIDs {
		node, ok := mapValue(workflow[nodeID])
		if !ok {
			continue
		}
		inputs, ok := mapValue(node["inputs"])
		if !ok {
			continue
		}
		title := ""
		if meta, ok := mapValue(node["_meta"]); ok {
			title = stringValue(meta["title"])
		}
		classType := stringValue(node["class_type"])
		if isCanvasAnnotationNode(classType) {
			continue
		}
		fieldNames := make([]string, 0, len(inputs))
		for fieldName := range inputs {
			fieldNames = append(fieldNames, fieldName)
		}
		sort.Strings(fieldNames)
		for _, fieldName := range fieldNames {
			fieldValue := inputs[fieldName]
			if isWorkflowLink(fieldValue) {
				continue
			}
			fieldType := discoveredFieldType(fieldName, fieldValue, classType)
			label := fieldName
			if title != "" && durationNodeTitle(title) {
				// H3 等模板把时长语义放在 PrimitiveFloat/PrimitiveInt 的节点标题里，
				// widget 输入名通常是 value；这里同步到稳定的 Bridge 参数来源。
				fieldType = "DURATION"
			}
			if title != "" {
				label = title + " · " + fieldName
			}
			field := jsonMap{"id": nodeID + "::" + fieldName, "nodeId": nodeID, "fieldName": fieldName, "classType": classType, "fieldValue": fieldValue, "fieldType": fieldType, "label": label, "enabled": true}
			if strings.EqualFold(fieldType, "DURATION") {
				// H3 等模板用 PrimitiveFloat 只携带当前时长，没有 widget 配置。
				// 这是 MiniMax H3 的官方有效范围；显式 min/max/options 仍然优先。
				field["min"] = 4
				field["max"] = 15
				field["step"] = 1
			}
			if (nodeID == promptNode && fieldName == promptField) || promptFields[nodeID+"::"+fieldName] {
				field["source"] = "prompt"
				field["required"] = true
			} else if source := discoveredDynamicSource(fieldName, fieldType, classType, capability); source != "" {
				field["source"] = source
			}
			switch normalizeSourceName(stringValue(field["source"])) {
			case "referenceimage":
				imageOrder++
				field["imageOrder"] = imageOrder
				field["sourceIndex"] = imageOrder - 1
				field["required"] = imageOrder == 1
				if role := workflowImageRole(field, title); role != "" {
					field["role"] = role
				}
				if branch := branches[nodeID]; branch != "" {
					field["mediaBranch"] = branch
				}
			case "referencevideo":
				field["sourceIndex"] = videoOrder
				field["required"] = false
				videoOrder++
			case "referenceaudio":
				field["sourceIndex"] = audioOrder
				field["required"] = audioOrder == 0
				audioOrder++
			case "mask":
				field["required"] = false
			}
			if isSeedField(fieldName) {
				field["randomEnabled"] = true
			}
			fields = append(fields, field)
		}
	}
	return fields
}

// promptTargetFields 返回模型节点的直接 prompt 输入。MiniMax H3 等工作流可能
// 同时保留 CLIPTextEncode 和模型节点内置 prompt，分支不同时只有一个可达，
// 因此这里允许多个 prompt 字段一起参与映射。
func promptTargetFields(workflow jsonMap, nodeIDs []string) map[string]bool {
	promptNode, promptField := findWorkflowPromptTarget(workflow, nodeIDs)
	result := make(map[string]bool)
	if promptNode != "" && promptField != "" {
		result[promptNode+"::"+promptField] = true
	}
	for _, nodeID := range nodeIDs {
		node, ok := mapValue(workflow[nodeID])
		if !ok {
			continue
		}
		inputs, ok := mapValue(node["inputs"])
		if !ok || !directModelPromptInput(stringValue(node["class_type"])) {
			continue
		}
		for fieldName, value := range inputs {
			if _, ok := value.(string); !ok {
				continue
			}
			if normalizeSourceName(fieldName) == "prompt" {
				result[nodeID+"::"+fieldName] = true
			}
		}
	}
	return result
}

func directModelPromptInput(classType string) bool {
	key := normalizeSourceName(classType)
	return stringContainsAny(key, "minimaxh3", "imagetovideo", "referencetovideo")
}

func workflowProducerGraph(workflow jsonMap) map[string][]string {
	graph := make(map[string][]string)
	for nodeID, raw := range workflow {
		node, ok := mapValue(raw)
		if !ok {
			continue
		}
		inputs, ok := mapValue(node["inputs"])
		if !ok {
			continue
		}
		for _, value := range inputs {
			link := sliceValue(value)
			if len(link) == 0 {
				continue
			}
			source := stringValue(link[0])
			if source == "" || source == nodeID {
				continue
			}
			graph[source] = append(graph[source], nodeID)
		}
	}
	return graph
}

// workflowImageBranches 按 LoadImage 的下游可达模型判断图片属于哪条视频分支。
// 整合工作流常把首尾帧和多参考画在同一张图里，按字段顺序分配会把两张图
// 抢给先声明的首尾帧分支，导致多参考节点拿不到图。
func workflowImageBranches(workflow jsonMap, nodeIDs []string, graph map[string][]string) map[string]string {
	branches := make(map[string]string)
	for _, nodeID := range nodeIDs {
		node, ok := mapValue(workflow[nodeID])
		if !ok || !strings.Contains(normalizeSourceName(stringValue(node["class_type"])), "loadimage") {
			continue
		}
		visited := make(map[string]bool)
		stack := append([]string{}, graph[nodeID]...)
		branch := ""
		for len(stack) > 0 {
			next := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[next] {
				continue
			}
			visited[next] = true
			node, ok := mapValue(workflow[next])
			if !ok {
				continue
			}
			classKey := normalizeSourceName(stringValue(node["class_type"]))
			if branch == "" && strings.Contains(classKey, "referencetovideo") {
				branch = "reference"
			} else if branch == "" && strings.Contains(classKey, "imagetovideo") {
				branch = "frames"
			}
			if branch != "" {
				break
			}
			stack = append(stack, graph[next]...)
		}
		if branch != "" {
			branches[nodeID] = branch
		}
	}
	return branches
}

// discoverWorkflowGraph 将 ComfyUI 画布 JSON 压缩成网页只读预览所需的节点和连线。
// 画布 JSON 不能直接提交给 /prompt，因此预览数据必须和可执行 API JSON 分开传输。
func discoverWorkflowGraph(workflow jsonMap) jsonMap {
	rawNodes := sliceValue(workflow["nodes"])
	if len(rawNodes) == 0 {
		return nil
	}

	nodes := make([]any, 0, len(rawNodes))
	nodeIDs := make(map[string]bool, len(rawNodes))
	for _, raw := range rawNodes {
		node, ok := mapValue(raw)
		if !ok {
			continue
		}
		nodeID := strings.TrimSpace(stringValue(node["id"]))
		if nodeID == "" || nodeIDs[nodeID] {
			continue
		}
		classType := firstNonEmpty(stringValue(node["type"]), stringValue(node["class_type"]), "Unknown")
		title := stringValue(node["title"])
		if title == "" {
			if properties, ok := mapValue(node["properties"]); ok {
				title = stringValue(properties["Node name for S&R"])
			}
		}
		if title == "" {
			title = classType
		}
		nodes = append(nodes, jsonMap{"id": nodeID, "title": title, "classType": classType})
		nodeIDs[nodeID] = true
	}
	if len(nodes) == 0 {
		return nil
	}

	edges := make([]any, 0)
	edgeKeys := make(map[string]bool)
	for _, raw := range sliceValue(workflow["links"]) {
		items := sliceValue(raw)
		if len(items) < 4 {
			continue
		}
		from := strings.TrimSpace(stringValue(items[1]))
		to := strings.TrimSpace(stringValue(items[3]))
		if from == "" || to == "" || from == to || !nodeIDs[from] || !nodeIDs[to] {
			continue
		}
		key := from + ":" + to
		if edgeKeys[key] {
			continue
		}
		edgeKeys[key] = true
		edges = append(edges, jsonMap{"from": from, "to": to})
	}
	return jsonMap{"nodes": nodes, "edges": edges}
}

// convertComfyCanvasWorkflow 将 ComfyUI 保存的画布 JSON 转成 /prompt 接受的 API JSON。
// 画布节点的 link 使用全局 link ID，API 节点则直接保存 [来源节点, 输出槽位]。
// 只转换画布中明确声明的输入和 widget 默认值，避免把布局字段带入执行请求。
func convertComfyCanvasWorkflow(workflow jsonMap) jsonMap {
	rawNodes := sliceValue(workflow["nodes"])
	if len(rawNodes) == 0 {
		return nil
	}
	links := make(map[string][]any)
	for _, raw := range sliceValue(workflow["links"]) {
		items := sliceValue(raw)
		if len(items) < 4 {
			continue
		}
		links[workflowNumberKey(items[0])] = items
	}

	converted := make(jsonMap, len(rawNodes))
	for _, raw := range rawNodes {
		node, ok := mapValue(raw)
		if !ok {
			continue
		}
		nodeID := strings.TrimSpace(stringValue(node["id"]))
		classType := strings.TrimSpace(firstNonEmpty(stringValue(node["type"]), stringValue(node["class_type"])))
		// 画布备注只用于编辑器展示，不是可执行节点。若将其转换进 /prompt，
		// ComfyUI 会按自定义节点校验，缺少 MarkdownNote 时直接让整个任务失败。
		if nodeID == "" || classType == "" || isCanvasAnnotationNode(classType) {
			continue
		}
		// 模板可能保留禁用的视频拆解或预览节点。加载器仍要留给动态素材，
		// 但这些节点直接转换后会让 ComfyUI 校验缺失的示例媒体。
		if isBypassedOptionalMediaConsumer(node, classType) {
			continue
		}
		inputs := make(jsonMap)
		namedWidgets := make(jsonMap)
		if rawNamed, ok := mapValue(node["widgets_values_named"]); ok {
			namedWidgets = rawNamed
		}
		widgetValues := sliceValue(node["widgets_values"])
		widgetIndex := 0
		for _, rawInput := range sliceValue(node["inputs"]) {
			input, ok := mapValue(rawInput)
			if !ok {
				continue
			}
			fieldName := strings.TrimSpace(stringValue(input["name"]))
			if fieldName == "" {
				continue
			}
			if linkKey := workflowNumberKey(input["link"]); linkKey != "" {
				if _, hasWidget := input["widget"]; hasWidget && widgetIndex < len(widgetValues) {
					widgetIndex++
				}
				if link, exists := links[linkKey]; exists {
					fromNode := strings.TrimSpace(stringValue(link[1]))
					fromSlot := int(numberValue(link[2]))
					if fromNode != "" {
						inputs[fieldName] = []any{fromNode, fromSlot}
					}
				}
				continue
			}
			widgetName := fieldName
			if widget, ok := mapValue(input["widget"]); ok {
				widgetName = firstNonEmpty(stringValue(widget["name"]), fieldName)
			}
			if value, exists := namedWidgets[widgetName]; exists {
				// UI 模板会保留未配置控件的 null。发送 null 会让 ComfyUI 的
				// 组合框校验失败，这里跳过并让其使用服务端默认值。
				if value != nil {
					inputs[fieldName] = value
				}
				widgetIndex++
				continue
			}
			if _, hasWidget := input["widget"]; hasWidget && widgetIndex < len(widgetValues) {
				if widgetValues[widgetIndex] != nil {
					inputs[fieldName] = widgetValues[widgetIndex]
				}
				widgetIndex++
			}
		}
		apiNode := jsonMap{"class_type": classType, "inputs": inputs}
		if title := firstNonEmpty(stringValue(node["title"]), nodeTitle(node)); title != "" {
			apiNode["_meta"] = jsonMap{"title": title}
		}
		if nodeModeIsBypassed(node) {
			apiNode["_canvasBypassed"] = true
		}
		converted[nodeID] = apiNode
	}
	if len(converted) == 0 {
		return nil
	}
	bypassUnconfiguredLoraNodes(converted)
	return converted
}

// 画布模板可能保留未选择 LoRA 的加载节点。API 模式不会自动补组合框默认值，
// 这类节点会让必填校验失败；这里把模型连接旁路到下游，保持模板其余部分可执行。
func bypassUnconfiguredLoraNodes(workflow jsonMap) {
	replacements := make(map[string][]any)
	invalid := make(map[string]bool)
	for nodeID, rawNode := range workflow {
		node, ok := rawNode.(jsonMap)
		if !ok || !isUnconfiguredLoraNode(node) {
			continue
		}
		inputs, ok := node["inputs"].(jsonMap)
		if !ok {
			invalid[nodeID] = true
			continue
		}
		modelLink, ok := inputs["model"].([]any)
		if !ok || len(modelLink) < 2 || stringValue(modelLink[0]) == "" {
			invalid[nodeID] = true
			continue
		}
		replacements[nodeID] = modelLink
	}
	if len(replacements) == 0 && len(invalid) == 0 {
		return
	}
	for nodeID := range replacements {
		delete(workflow, nodeID)
	}
	for nodeID := range invalid {
		delete(workflow, nodeID)
	}
	for _, rawNode := range workflow {
		node, ok := rawNode.(jsonMap)
		if !ok {
			continue
		}
		inputs, ok := node["inputs"].(jsonMap)
		if !ok {
			continue
		}
		for fieldName, value := range inputs {
			link, ok := value.([]any)
			if !ok || len(link) == 0 {
				continue
			}
			if replacement, exists := replacements[stringValue(link[0])]; exists {
				inputs[fieldName] = append([]any{}, replacement...)
			}
		}
	}
}

func isUnconfiguredLoraNode(node jsonMap) bool {
	classType := strings.ToLower(firstNonEmpty(stringValue(node["class_type"]), stringValue(node["type"])))
	if classType != "loraloader" && classType != "loraloadermodelonly" {
		return false
	}
	inputs, ok := node["inputs"].(jsonMap)
	if !ok {
		return true
	}
	_, hasLora := inputs["lora_name"]
	return !hasLora
}

func isCanvasAnnotationNode(classType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(classType))
	return normalized == "note" ||
		strings.HasPrefix(normalized, "note:") ||
		normalized == "markdownnote" ||
		strings.HasPrefix(normalized, "markdownnote:") ||
		// rgthree labels are canvas-only headings and must not be sent to /prompt.
		normalized == "label (rgthree)" ||
		normalized == "label" ||
		normalized == "addlabel" ||
		normalized == "easy showanything" ||
		normalized == "fast groups bypasser (rgthree)" ||
		normalized == "fast groups bypasser"
}

func stripCanvasAnnotationNodes(workflow jsonMap) {
	for nodeID, raw := range workflow {
		node, ok := mapValue(raw)
		if !ok || !isCanvasAnnotationNode(firstNonEmpty(stringValue(node["class_type"]), stringValue(node["type"]))) {
			continue
		}
		delete(workflow, nodeID)
	}
}

func nodeTitle(node jsonMap) string {
	if properties, ok := mapValue(node["properties"]); ok {
		return stringValue(properties["Node name for S&R"])
	}
	return ""
}

func workflowNumberKey(value any) string {
	if value == nil || strings.TrimSpace(stringValue(value)) == "" {
		return ""
	}
	return strconv.FormatInt(int64(numberValue(value)), 10)
}

func sortedNodeIDs(workflow jsonMap) []string {
	items := make([]string, 0, len(workflow))
	for nodeID := range workflow {
		items = append(items, nodeID)
	}
	sort.Slice(items, func(i, j int) bool {
		left, leftErr := strconv.ParseInt(items[i], 10, 64)
		right, rightErr := strconv.ParseInt(items[j], 10, 64)
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		return items[i] < items[j]
	})
	return items
}

func findWorkflowPromptTarget(workflow jsonMap, nodeIDs []string) (string, string) {
	bestNode, bestField, bestScore := "", "", math.MinInt
	bestUserPrompt, userField, bestUserScore := "", "", math.MinInt
	for _, nodeID := range nodeIDs {
		node, ok := mapValue(workflow[nodeID])
		if !ok {
			continue
		}
		inputs, ok := mapValue(node["inputs"])
		if !ok {
			continue
		}
		classType := strings.ToLower(stringValue(node["class_type"]))
		title := ""
		if meta, ok := mapValue(node["_meta"]); ok {
			title = strings.ToLower(stringValue(meta["title"]))
		}
		for fieldName, value := range inputs {
			if _, ok := value.(string); !ok || !isPromptCandidate(fieldName, classType, title) {
				continue
			}
			descriptor := strings.ToLower(fieldName + " " + title)
			score := 0
			if strings.Contains(descriptor, "negative") || strings.Contains(descriptor, "负面") || strings.Contains(descriptor, "反向") || strings.Contains(descriptor, "负向") {
				score -= 100
			}
			if strings.Contains(descriptor, "positive") || strings.Contains(descriptor, "正面") || strings.Contains(descriptor, "正向") {
				score += 20
			}
			if strings.Contains(classType, "cliptextencode") {
				score += 5
			}
			if isPromptWorkflowField(jsonMap{"fieldName": fieldName}) {
				score += 2
			}
			if directModelPromptInput(classType) {
				score += 50
			}
			// 常见模板会把“用户输入”放在 User Prompt、把系统规则放在 System Prompt。
			// 用户提示词必须优先绑定到 User Prompt，否则测试输入只会替换生成规则。
			isUserPrompt := strings.Contains(descriptor, "user prompt") ||
				strings.Contains(normalizeSourceName(fieldName), "userprompt") ||
				strings.Contains(title, "用户提示词") ||
				strings.Contains(title, "用户输入")
			if isUserPrompt {
				score += 40
			}
			if bestNode == "" || score > bestScore {
				bestNode, bestField, bestScore = nodeID, fieldName, score
			}
			if isUserPrompt && score > bestUserScore {
				bestUserPrompt, userField, bestUserScore = nodeID, fieldName, score
			}
		}
	}
	if bestUserPrompt != "" {
		return bestUserPrompt, userField
	}
	return bestNode, bestField
}

func isPromptCandidate(fieldName, classType, title string) bool {
	key := normalizeSourceName(fieldName)
	if stringIn(key, "text", "prompt", "positiveprompt", "positive", "caption", "description") {
		return true
	}
	if strings.ToLower(fieldName) != "value" {
		return false
	}
	descriptor := classType + " " + title
	return strings.Contains(descriptor, "cliptextencode") || strings.Contains(descriptor, "text") || strings.Contains(descriptor, "string") || strings.Contains(descriptor, "prompt") || strings.Contains(descriptor, "提示词") || strings.Contains(descriptor, "文本")
}

func discoveredFieldType(fieldName string, value any, classType string) string {
	key, classKey := normalizeSourceName(fieldName), normalizeSourceName(classType)
	if strings.Contains(key, "mask") {
		return "IMAGE"
	}
	if strings.Contains(classKey, "loadimage") && stringIn(key, "image", "file", "path") {
		return "IMAGE"
	}
	if strings.Contains(classKey, "loadvideo") && stringIn(key, "video", "file", "path") {
		return "VIDEO"
	}
	if strings.Contains(classKey, "loadaudio") && stringIn(key, "audio", "file", "path") {
		return "AUDIO"
	}
	if discoveredNamedDynamicSource(fieldName, classType, "") != "" {
		return scalarFieldType(value)
	}
	switch value.(type) {
	case bool:
		return "BOOLEAN"
	case float64, float32, int, int64:
		return "NUMBER"
	}
	if mediaFieldName(key, "image") || mediaValueType(value) == "IMAGE" {
		return "IMAGE"
	}
	if mediaFieldName(key, "video") || mediaValueType(value) == "VIDEO" {
		return "VIDEO"
	}
	if mediaFieldName(key, "audio") || mediaValueType(value) == "AUDIO" {
		return "AUDIO"
	}
	return "TEXT"
}

func mediaValueType(value any) string {
	if items := sliceValue(value); len(items) > 0 {
		value = items[0]
	}
	text := strings.ToLower(stringValue(value))
	if index := strings.IndexAny(text, "?#"); index >= 0 {
		text = text[:index]
	}
	if hasAnySuffix(text, ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".avif") {
		return "IMAGE"
	}
	if hasAnySuffix(text, ".mp4", ".webm", ".mov", ".m4v", ".mkv") {
		return "VIDEO"
	}
	if hasAnySuffix(text, ".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac") {
		return "AUDIO"
	}
	return ""
}

func discoveredDynamicSource(fieldName, fieldType, classType, capability string) string {
	if source := discoveredNamedDynamicSource(fieldName, classType, capability); source != "" {
		return source
	}
	if strings.EqualFold(strings.TrimSpace(fieldType), "DURATION") {
		return "videoSeconds"
	}
	switch strings.ToUpper(strings.TrimSpace(fieldType)) {
	case "IMAGE":
		return "referenceImage"
	case "VIDEO":
		return "referenceVideo"
	case "AUDIO":
		return "referenceAudio"
	default:
		return ""
	}
}

func discoveredNamedDynamicSource(fieldName, classType, capability string) string {
	key := normalizeSourceName(fieldName)
	classKey := normalizeSourceName(classType)
	if classKey == "resolutionselector" {
		if key == "aspectratio" {
			return "aspectRatio"
		}
		if key == "megapixels" {
			return "vquality"
		}
	}
	if strings.Contains(key, "mask") {
		return "mask"
	}
	if source := dimensionFieldSource(key); source != "" {
		return source
	}
	if stringIn(key, "ratio", "aspectratio", "imageaspectratio", "imageratio", "videoaspectratio", "videoratio") {
		return "aspectRatio"
	}
	if stringIn(key, "videoresolution", "videoquality", "vquality") {
		return "vquality"
	}
	if stringIn(key, "size", "imagesize", "imageresolution") {
		return "size"
	}
	if key == "resolution" {
		if capability == "video" {
			return "vquality"
		}
		if capability == "image" {
			return "size"
		}
	}
	aliases := map[string]string{
		"batch": "count", "batchsize": "count", "count": "count", "numimages": "count", "numberofimages": "count", "imagecount": "count", "imagescount": "count",
		"quality": "quality", "duration": "videoSeconds", "seconds": "videoSeconds", "durationseconds": "videoSeconds", "videoseconds": "videoSeconds", "videoduration": "videoSeconds", "videodurationseconds": "videoSeconds", "videolength": "videoSeconds", "clipduration": "videoSeconds",
		"generateaudio": "videoGenerateAudio", "videogenerateaudio": "videoGenerateAudio", "watermark": "videoWatermark", "videowatermark": "videoWatermark",
		"audioformat": "audioFormat", "voice": "audioVoice", "audiovoice": "audioVoice", "audiospeed": "audioSpeed", "audioinstructions": "audioInstructions",
		"transparent": "transparentBackground", "transparentbackground": "transparentBackground",
	}
	return aliases[key]
}

func durationNodeTitle(title string) bool {
	key := normalizeSourceName(title)
	return (strings.Contains(key, "duration") || strings.Contains(key, "seconds") ||
		strings.Contains(key, "时长") || strings.Contains(key, "秒数")) &&
		!strings.Contains(key, "frame") && !strings.Contains(key, "帧")
}

var dimensionPrefixes = []string{"", "image", "video", "size", "output", "target", "latent", "frame", "canvas", "source", "resolution", "final"}

func dimensionFieldSource(key string) string {
	for _, prefix := range dimensionPrefixes {
		if key == prefix+"width" {
			return "width"
		}
		if key == prefix+"height" {
			return "height"
		}
	}
	if key == "pixelwidth" || key == "widthpixels" {
		return "width"
	}
	if key == "pixelheight" || key == "heightpixels" {
		return "height"
	}
	return ""
}

func scalarFieldType(value any) string {
	switch value.(type) {
	case bool:
		return "BOOLEAN"
	case float64, float32, int, int64:
		return "NUMBER"
	default:
		return "TEXT"
	}
}

func mediaFieldName(key, mediaType string) bool {
	if key == mediaType {
		return true
	}
	for _, prefix := range []string{"input", "reference", "ref", "source", "init", "start", "end", "first", "last", "control", "controlnet", "style", "subject"} {
		if key == prefix+mediaType {
			return true
		}
	}
	for _, suffix := range []string{"file", "path", "filename", "upload"} {
		if key == mediaType+suffix {
			return true
		}
	}
	return mediaType == "image" && stringIn(key, "firstframe", "lastframe", "startframe", "endframe")
}

func isWorkflowLink(value any) bool {
	items := sliceValue(value)
	if len(items) != 2 || stringValue(items[0]) == "" {
		return false
	}
	index := numberValue(items[1])
	return index == math.Trunc(index)
}

func isSeedField(value string) bool {
	key := normalizeSourceName(value)
	return key == "seed" || strings.HasSuffix(key, "seed") || strings.Contains(key, "noiseseed")
}

func normalizeSourceName(value string) string {
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
}

func validateWorkflowMediaInputs(fields []any, payload jsonMap) error {
	capacities := map[string]int{"referencevideo": 0, "referenceaudio": 0}
	hasMask := false
	referenceFields := make([]any, 0)
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok || field["enabled"] == false {
			continue
		}
		source := normalizedWorkflowFieldSource(field)
		if source == "mask" {
			hasMask = true
			continue
		}
		key := canonicalMediaSource(source)
		if _, ok := capacities[key]; !ok {
			if key == "referenceimage" {
				referenceFields = append(referenceFields, raw)
			}
			continue
		}
		index := workflowSourceIndex(field)
		if index+1 > capacities[key] {
			capacities[key] = index + 1
		}
	}
	capacities["referenceimage"] = effectiveReferenceImageCapacity(referenceFields, payload)
	checks := []struct {
		label string
		key   string
		value any
	}{{"参考图片", "referenceimage", payload["referenceImages"]}, {"参考视频", "referencevideo", payload["referenceVideos"]}, {"参考音频", "referenceaudio", payload["referenceAudios"]}}
	for _, check := range checks {
		count := len(sliceValue(check.value))
		if count > capacities[check.key] {
			return fmt.Errorf("工作流只配置了 %d 个%s槽位，但画布传入了 %d 个；请在字段映射中补齐槽位", capacities[check.key], check.label, count)
		}
	}
	if payload["mask"] != nil && !hasMask {
		return errors.New("画布传入了蒙版，但工作流没有配置蒙版字段映射")
	}
	return nil
}

// effectiveReferenceImageCapacity 与图片分配使用同一套分支过滤。UI 模板常保留
// 首尾帧和未连接的 LoadImage 示例节点；多参考模式下它们不会真正消费素材，
// 提交前不能把这类展示节点算成可用槽位。
func effectiveReferenceImageCapacity(fields []any, payload jsonMap) int {
	allocation := newReferenceImageAllocation(payload)
	capacity := 0
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok || normalizedWorkflowFieldSource(field) != "referenceimage" {
			continue
		}
		branch := stringValue(field["mediaBranch"])
		if allocation.preferredBranch != "" && branch != "" && branch != allocation.preferredBranch {
			continue
		}
		if allocation.preferredBranch == "reference" && allocation.hasDirectReference && !isDirectReferenceImageField(field) {
			continue
		}
		capacity++
	}
	return capacity
}

func applyWorkflowFields(workflow jsonMap, payload jsonMap, files map[string]string) error {
	fields := sliceValue(payload["workflowFields"])
	imageAllocation := newReferenceImageAllocation(payload)
	removed := map[string]bool{}
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok || field["enabled"] == false {
			continue
		}
		nodeID := firstNonEmpty(stringValue(field["nodeId"]), stringValue(field["node"]))
		fieldName := firstNonEmpty(stringValue(field["fieldName"]), stringValue(field["input"]))
		if nodeID == "" || fieldName == "" {
			continue
		}
		node, ok := mapValue(workflow[nodeID])
		if !ok {
			if boolValue(field["required"]) {
				return fmt.Errorf("工作流缺少必填映射节点 %s", nodeID)
			}
			continue
		}
		source := normalizedWorkflowFieldSource(field)
		if nodeModeIsBypassed(node) && isMediaSource(source) {
			inputs, inputsOK := mapValue(node["inputs"])
			if !inputsOK {
				inputs = jsonMap{}
			}
			fieldName := firstNonEmpty(stringValue(field["fieldName"]), stringValue(field["input"]))
			if fieldName != "" {
				delete(inputs, fieldName)
				node["inputs"] = inputs
			}
		}
		inputs, ok := mapValue(node["inputs"])
		if !ok {
			inputs = jsonMap{}
		}
		value := firstValue(field["value"], field["fieldValue"], field["default"])
		if source != "" {
			value = resolveSource(source, field, payload, files, imageAllocation)
		} else if boolValue(field["randomEnabled"]) || boolValue(field["random_enabled"]) {
			var err error
			value, err = randomWorkflowValue(field)
			if err != nil {
				return err
			}
		}
		if !emptyWorkflowValue(value) {
			inputs[fieldName] = normalizeComfyValue(value)
		} else if boolValue(field["required"]) {
			return fmt.Errorf("工作流必填字段 %s.%s 缺少值", nodeID, fieldName)
		} else if isMediaSource(normalizedWorkflowFieldSource(field)) {
			delete(inputs, fieldName)
		}
		node["inputs"] = inputs
	}
	emptyMediaLoaders, removedEmpty := removeEmptyMediaLoaderNodes(workflow, fields)
	for nodeID := range emptyMediaLoaders {
		removed[nodeID] = true
	}
	if !removedEmpty && len(removed) == 0 {
		return nil
	}
	for nodeID := range removed {
		delete(workflow, nodeID)
	}
	for _, rawNode := range workflow {
		node, ok := mapValue(rawNode)
		if !ok {
			continue
		}
		inputs, ok := mapValue(node["inputs"])
		if !ok {
			continue
		}
		for fieldName, value := range inputs {
			link := sliceValue(value)
			if len(link) > 0 && removed[stringValue(link[0])] {
				delete(inputs, fieldName)
			}
		}
	}
	// 旁路节点可能在转换阶段就已移除，但它们仍会留下指向不存在节点的输出。
	// ComfyUI 的递归校验会把这些悬空 link 一起执行，所以提交前必须统一清理。
	for _, rawNode := range workflow {
		node, ok := mapValue(rawNode)
		if !ok {
			continue
		}
		inputs, ok := mapValue(node["inputs"])
		if !ok {
			continue
		}
		for fieldName, value := range inputs {
			if !isWorkflowLink(value) {
				continue
			}
			source := stringValue(sliceValue(value)[0])
			if _, exists := workflow[source]; !exists {
				delete(inputs, fieldName)
			}
		}
	}
	// 禁用/旁路的展示与拆解节点常只是为了让前端能预览模板里的示例媒体。
	// 它们的媒体来源被删除后不应继续提交，否则 ComfyUI 会把这些旁路节点
	// 中的 required input 也纳入校验。
	for nodeID, rawNode := range workflow {
		node, ok := mapValue(rawNode)
		if !ok || !isOptionalMediaConsumer(node) {
			continue
		}
		inputs, ok := mapValue(node["inputs"])
		if !ok {
			continue
		}
		missingRequiredMedia := false
		for fieldName, value := range inputs {
			if !isWorkflowLink(value) {
				continue
			}
			source := stringValue(sliceValue(value)[0])
			if !removed[source] {
				continue
			}
			delete(inputs, fieldName)
			missingRequiredMedia = true
		}
		if missingRequiredMedia {
			removed[nodeID] = true
			delete(workflow, nodeID)
		}
	}
	return nil
}

// removeEmptyMediaLoaderNodes 清理未被画布素材填充的 LoadImage/LoadVideo/LoadAudio。
// 这类节点常带 upload 等辅助控件；只删除媒体输入会留下空节点，导致 ComfyUI
// 仍尝试读取模板中的示例文件，或把下游 ref_image 槽位当成悬空输入。
func removeEmptyMediaLoaderNodes(workflow jsonMap, fields []any) (map[string]bool, bool) {
	fieldSources := make(map[string]jsonMap)
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok {
			continue
		}
		nodeID := firstNonEmpty(stringValue(field["nodeId"]), stringValue(field["node"]))
		fieldName := firstNonEmpty(stringValue(field["fieldName"]), stringValue(field["input"]))
		if nodeID == "" || fieldName == "" {
			continue
		}
		key := nodeID + "::" + fieldName
		fieldSources[key] = field
	}

	removed := map[string]bool{}
	for nodeID, rawNode := range workflow {
		node, ok := mapValue(rawNode)
		if !ok || !isMediaLoaderNode(node) {
			continue
		}
		inputs, ok := mapValue(node["inputs"])
		if !ok {
			continue
		}
		classKey := normalizeSourceName(stringValue(node["class_type"]))
		hasMedia := false
		for fieldName, value := range inputs {
			if !isMediaLoaderPrimaryInput(classKey, normalizeSourceName(fieldName)) {
				continue
			}
			if isWorkflowLink(value) {
				hasMedia = true
				continue
			}
			field := fieldSources[nodeID+"::"+fieldName]
			if !emptyWorkflowValue(value) && boolValue(field["enabled"]) {
				hasMedia = true
				continue
			}
			if enabled, exists := field["enabled"]; exists && enabled == false {
				hasMedia = true
				continue
			}
			if hasMediaLoaderFilename(value) {
				hasMedia = true
			}
		}
		if !hasMedia {
			removed[nodeID] = true
		}
	}
	for nodeID := range removed {
		delete(workflow, nodeID)
	}
	for _, rawNode := range workflow {
		node, ok := mapValue(rawNode)
		if !ok {
			continue
		}
		delete(node, "_canvasBypassed")
	}
	return removed, len(removed) > 0
}

func isMediaLoaderNode(node jsonMap) bool {
	classKey := normalizeSourceName(stringValue(node["class_type"]))
	return stringContainsAny(classKey, "loadimage", "loadvideo", "loadaudio")
}

func isMediaLoaderPrimaryInput(classKey, fieldName string) bool {
	switch {
	case strings.Contains(classKey, "loadimage"):
		return fieldName == "image"
	case strings.Contains(classKey, "loadvideo"):
		return fieldName == "file" || fieldName == "video"
	case strings.Contains(classKey, "loadaudio"):
		return fieldName == "audio"
	default:
		return false
	}
}

func isOptionalMediaConsumer(node jsonMap) bool {
	return nodeModeIsBypassed(node) && isOptionalMediaConsumerClassType(stringValue(node["class_type"]))
}

func isBypassedOptionalMediaConsumer(node jsonMap, classType string) bool {
	return nodeModeIsBypassed(node) && isOptionalMediaConsumerClassType(classType)
}

func nodeModeIsBypassed(node jsonMap) bool {
	switch int(numberValue(node["mode"])) {
	case 2, 4:
	default:
		return false
	}
	return true
}

func isOptionalMediaConsumerClassType(classType string) bool {
	classKey := normalizeSourceName(classType)
	return stringContainsAny(classKey,
		"getvideocomponents", "getimagecomponents", "getaudiocomponents",
		"showanything", "showtext", "previewimage", "previewvideo", "previewaudio",
	)
}

func hasMediaLoaderFilename(value any) bool {
	text := stringValue(value)
	if text == "" {
		return false
	}
	return hasAnySuffix(text,
		".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".avif",
		".mp4", ".webm", ".mov", ".m4v", ".mkv",
		".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac")
}

func normalizedWorkflowFieldSource(field jsonMap) string {
	source := stringValue(field["source"])
	if source == "" && (boolValue(field["bindPrompt"]) || boolValue(field["bind_prompt"])) {
		source = "prompt"
	}
	if source == "" && (boolValue(field["sourceFromUpstream"]) || boolValue(field["source_from_upstream"])) {
		fieldType := strings.ToLower(firstNonEmpty(stringValue(field["fieldType"]), stringValue(field["type"])))
		switch fieldType {
		case "image":
			source = "referenceImage"
		case "video":
			source = "referenceVideo"
		case "audio":
			source = "referenceAudio"
		default:
			if isPromptWorkflowField(field) {
				source = "prompt"
			}
		}
	}
	return normalizeSourceName(source)
}

func randomWorkflowValue(field jsonMap) (int64, error) {
	minimum, err := integerFieldBound(field["min"], 0)
	if err != nil {
		return 0, err
	}
	maximum, err := integerFieldBound(field["max"], math.MaxInt32)
	if err != nil {
		return 0, err
	}
	if maximum < minimum {
		return 0, errors.New("工作流随机值最大值不能小于最小值")
	}
	return minimum + rand.Int64N(maximum-minimum+1), nil
}

func integerFieldBound(value any, fallback int64) (int64, error) {
	if value == nil || stringValue(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(strings.Trim(stringValue(value), `"`), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("工作流随机值范围不是有效整数：%s", stringValue(value))
	}
	return parsed, nil
}

func resolveSource(source string, field, payload jsonMap, files map[string]string, imageAllocation *referenceImageAllocation) any {
	normalized := normalizeSourceName(source)
	if stringIn(normalized, "prompt", "text", "positiveprompt", "positive") {
		return stringValue(payload["prompt"])
	}
	params, _ := mapValue(payload["params"])
	switch {
	case stringIn(normalized, "size", "imagesize", "imageresolution"):
		return params["size"]
	case normalized == "resolution":
		if strings.ToLower(stringValue(payload["mode"])) == "video" {
			return videoResolutionValue(field, params["vquality"], stringValue(params["size"]))
		}
		return params["size"]
	case stringIn(normalized, "aspectratio", "ratio", "imageaspectratio", "imageratio", "videoaspectratio", "videoratio"):
		return aspectRatioValue(field, params["size"])
	case stringIn(normalized, "sizewidth", "width", "imagewidth", "videowidth"):
		return dimensionPart(stringValue(payload["mode"]), stringValue(params["size"]), stringValue(params["vquality"]), 0)
	case stringIn(normalized, "sizeheight", "height", "imageheight", "videoheight"):
		return dimensionPart(stringValue(payload["mode"]), stringValue(params["size"]), stringValue(params["vquality"]), 1)
	}
	aliases := map[string]string{
		"quality": "quality", "count": "count", "batch": "count", "batchsize": "count", "videoseconds": "videoSeconds", "duration": "videoSeconds",
		"videogenerateaudio": "videoGenerateAudio", "generateaudio": "videoGenerateAudio", "videowatermark": "videoWatermark", "watermark": "videoWatermark",
		"audioformat": "audioFormat", "systemprompt": "systemPrompt", "transparentbackground": "transparentBackground", "audiovoice": "audioVoice", "voice": "audioVoice", "audiospeed": "audioSpeed", "audioinstructions": "audioInstructions",
	}
	if stringIn(normalized, "vquality", "videoquality", "videoresolution") {
		return videoResolutionValue(field, params["vquality"], stringValue(params["size"]))
	}
	if key := aliases[normalized]; key != "" {
		return params[key]
	}
	if normalized == "mask" {
		media, _ := mapValue(payload["mask"])
		return files[stringValue(media["id"])]
	}
	values := payload["referenceImages"]
	if strings.Contains(normalized, "video") {
		values = payload["referenceVideos"]
	} else if strings.Contains(normalized, "audio") {
		values = payload["referenceAudios"]
	}
	items := sliceValue(values)
	if canonicalMediaSource(normalized) == "referenceimage" {
		if imageAllocation == nil {
			imageAllocation = newReferenceImageAllocation(payload)
		}
		if media := imageAllocation.resolve(field, items); media != nil {
			return files[stringValue(media["id"])]
		}
		return nil
	}
	index := workflowSourceIndex(field)
	if index < 0 || index >= len(items) {
		return nil
	}
	media, _ := mapValue(items[index])
	return files[stringValue(media["id"])]
}

type referenceImageAllocation struct {
	startID               string
	endID                 string
	hasExplicitFrameRoles bool
	preferredBranch       string
	hasDirectReference    bool
	used                  map[string]bool
}

func newReferenceImageAllocation(payload jsonMap) *referenceImageAllocation {
	metadata, _ := mapValue(payload["metadata"])
	startID := ""
	endID := ""
	hasExplicitFrameRoles := false
	preferredBranch := ""
	hasDirectReference := false
	startID = firstNonEmpty(stringValue(metadata["videoStartFrameNodeId"]), stringValue(metadata["video_start_frame_node_id"]))
	endID = firstNonEmpty(stringValue(metadata["videoEndFrameNodeId"]), stringValue(metadata["video_end_frame_node_id"]))
	switch strings.ToLower(strings.TrimSpace(stringValue(metadata["videoEditOperation"]))) {
	case "reference_to_video":
		preferredBranch = "reference"
	case "image_to_video", "start_end_to_video":
		preferredBranch = "frames"
	}
	fields := sliceValue(payload["workflowFields"])
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok {
			continue
		}
		if role := normalizeWorkflowImageRole(stringValue(field["role"])); role == "first_frame" || role == "last_frame" {
			hasExplicitFrameRoles = true
		}
		if normalizedWorkflowFieldSource(field) == "referenceimage" &&
			stringValue(field["mediaBranch"]) == "reference" &&
			isDirectReferenceImageField(field) {
			hasDirectReference = true
		}
	}
	items := sliceValue(payload["referenceImages"])
	if startID != "" || endID != "" {
		startID, endID = inferReferenceImageFrameIDs(items, startID, endID)
	}
	return &referenceImageAllocation{startID: startID, endID: endID, hasExplicitFrameRoles: hasExplicitFrameRoles, preferredBranch: preferredBranch, hasDirectReference: hasDirectReference, used: map[string]bool{}}
}

func (allocation *referenceImageAllocation) resolve(field jsonMap, items []any) jsonMap {
	if branch := stringValue(field["mediaBranch"]); allocation.preferredBranch != "" && branch != "" && branch != allocation.preferredBranch {
		return nil
	}
	if allocation.preferredBranch == "reference" && allocation.hasDirectReference && !isDirectReferenceImageField(field) {
		return nil
	}
	role := normalizeWorkflowImageRole(stringValue(field["role"]))
	switch role {
	case "first_frame":
		return allocation.claim(allocation.startID, items)
	case "last_frame":
		return allocation.claim(allocation.endID, items)
	case "reference_image":
		return allocation.claim("", items)
	}
	return allocation.claim("", items)
}

func isDirectReferenceImageField(field jsonMap) bool {
	return strings.Contains(normalizeSourceName(stringValue(field["classType"])), "referencetovideo") &&
		strings.Contains(normalizeSourceName(stringValue(field["fieldName"])), "refimage")
}

func (allocation *referenceImageAllocation) claim(preferredID string, items []any) jsonMap {
	if preferredID != "" {
		for _, raw := range items {
			media, ok := mapValue(raw)
			if !ok || stringValue(media["id"]) != preferredID || allocation.used[preferredID] {
				continue
			}
			allocation.used[preferredID] = true
			return media
		}
	}
	for _, raw := range items {
		media, ok := mapValue(raw)
		if !ok {
			continue
		}
		id := stringValue(media["id"])
		if id == "" || allocation.used[id] {
			continue
		}
		if preferredID == "" && allocation.hasExplicitFrameRoles && (id == allocation.startID || id == allocation.endID) {
			continue
		}
		allocation.used[id] = true
		return media
	}
	return nil
}

func inferReferenceImageFrameIDs(items []any, startID, endID string) (string, string) {
	if len(items) == 0 {
		return startID, endID
	}
	first, _ := mapValue(items[0])
	if startID == "" {
		startID = stringValue(first["id"])
	}
	if len(items) > 1 && endID == "" {
		last, _ := mapValue(items[1])
		endID = stringValue(last["id"])
	}
	return startID, endID
}

func mediaByID(items []any, id string) jsonMap {
	for _, raw := range items {
		media, ok := mapValue(raw)
		if ok && stringValue(media["id"]) == id {
			return media
		}
	}
	return nil
}

func workflowImageRole(field jsonMap, title string) string {
	descriptor := normalizeSourceName(strings.Join([]string{
		stringValue(field["fieldName"]),
		stringValue(field["classType"]),
		title,
	}, " "))
	return normalizeWorkflowImageRole(descriptor)
}

func normalizeWorkflowImageRole(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if stringContainsAny(value, "first_frame", "firstframe", "start_frame", "startframe", "start_image", "startimage", "首帧", "起始帧", "开始帧") {
		return "first_frame"
	}
	if stringContainsAny(value, "last_frame", "lastframe", "end_frame", "endframe", "end_image", "endimage", "尾帧", "结束帧", "末帧", "终帧") {
		return "last_frame"
	}
	if value == "reference_image" {
		return "reference_image"
	}
	return ""
}

func stringContainsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func applyPromptFallback(workflow jsonMap, prompt string, fields []any) {
	if prompt == "" {
		return
	}
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok {
			continue
		}
		source := normalizedWorkflowFieldSource(field)
		if boolValue(field["bind_prompt"]) || stringIn(source, "prompt", "text", "positiveprompt", "positive") {
			return
		}
	}
	nodeID, fieldName := findWorkflowPromptTarget(workflow, sortedNodeIDs(workflow))
	node, ok := mapValue(workflow[nodeID])
	if !ok {
		return
	}
	inputs, ok := mapValue(node["inputs"])
	if ok {
		inputs[fieldName] = prompt
	}
}

func isPromptWorkflowField(field jsonMap) bool {
	for _, key := range []string{"fieldName", "input", "label", "name"} {
		if stringIn(normalizeSourceName(stringValue(field[key])), "prompt", "text", "positiveprompt", "caption", "description", "提示词", "正向提示词") {
			return true
		}
	}
	return false
}

func normalizeComfyValue(value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	trimmed := strings.TrimSpace(text)
	if strings.EqualFold(trimmed, "true") {
		return true
	}
	if strings.EqualFold(trimmed, "false") {
		return false
	}
	if numberPattern.MatchString(trimmed) {
		if number, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return number
		}
	}
	return value
}

func emptyWorkflowValue(value any) bool {
	return value == nil || (func() bool { text, ok := value.(string); return ok && strings.TrimSpace(text) == "" })()
}

func workflowSourceIndex(field jsonMap) int {
	if order := int(numberValue(field["imageOrder"])); order > 0 {
		return order - 1
	}
	index := int(numberValue(field["sourceIndex"]))
	if index < 0 {
		return 0
	}
	return index
}

func canonicalMediaSource(source string) string {
	switch source {
	case "image", "referenceimages":
		return "referenceimage"
	case "video", "referencevideos":
		return "referencevideo"
	case "audio", "referenceaudios":
		return "referenceaudio"
	default:
		return source
	}
}

func isMediaSource(source string) bool {
	return stringIn(source, "referenceimage", "referenceimages", "image", "referencevideo", "referencevideos", "video", "referenceaudio", "referenceaudios", "audio", "mask")
}

func dimensionPart(mode, size, quality string, index int) any {
	if pixels, ok := pixelDimensions(size); ok {
		return pixels[index]
	}
	var dimensions [2]int
	var ok bool
	if strings.EqualFold(strings.TrimSpace(mode), "video") {
		dimensions, ok = videoDimensions(size, quality)
	} else {
		dimensions, ok = imageDimensions(size)
	}
	if !ok {
		return nil
	}
	return roundStep(dimensions[index], 16)
}

func pixelDimensions(value string) ([2]int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), "x")
	if len(parts) != 2 {
		return [2]int{}, false
	}
	width, errWidth := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errHeight := strconv.Atoi(strings.TrimSpace(parts[1]))
	return [2]int{width, height}, errWidth == nil && errHeight == nil && width > 0 && height > 0
}

func aspectRatio(value string) any {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if match := regexp.MustCompile(`^(\d+)\s*:\s*(\d+)`).FindStringSubmatch(normalized); len(match) == 3 {
		width, widthErr := strconv.Atoi(match[1])
		height, heightErr := strconv.Atoi(match[2])
		if widthErr == nil && heightErr == nil && width > 0 && height > 0 {
			divisor := gcd(width, height)
			return fmt.Sprintf("%d:%d", width/divisor, height/divisor)
		}
	}
	normalized = regexp.MustCompile(`-(1k|2k|4k)$`).ReplaceAllString(normalized, "")
	if parts, ok := ratioParts(normalized); ok {
		divisor := gcd(parts[0], parts[1])
		return fmt.Sprintf("%d:%d", parts[0]/divisor, parts[1]/divisor)
	}
	dimensions, ok := pixelDimensions(normalized)
	if !ok {
		return nil
	}
	actual := float64(dimensions[0]) / float64(dimensions[1])
	known := [][2]int{{1, 1}, {3, 2}, {2, 3}, {4, 3}, {3, 4}, {4, 5}, {5, 4}, {16, 9}, {9, 16}, {2, 1}, {1, 2}, {21, 9}}
	best, difference := [2]int{}, math.Inf(1)
	for _, candidate := range known {
		expected := float64(candidate[0]) / float64(candidate[1])
		current := math.Abs(actual-expected) / expected
		if current < difference {
			best, difference = candidate, current
		}
	}
	if difference <= 0.03 {
		return fmt.Sprintf("%d:%d", best[0], best[1])
	}
	divisor := gcd(dimensions[0], dimensions[1])
	return fmt.Sprintf("%d:%d", dimensions[0]/divisor, dimensions[1]/divisor)
}

// aspectRatioValue keeps the canonical canvas ratio while restoring the exact
// enum value expected by ComfyUI nodes such as ResolutionSelector.
func aspectRatioValue(field jsonMap, value any) any {
	canonical, _ := aspectRatio(stringValue(value)).(string)
	if canonical == "" {
		return value
	}
	candidates := sliceValue(field["options"])
	if isResolutionSelectorField(field, "aspectratio") {
		for _, option := range resolutionSelectorAspectRatioOptions {
			candidates = append(candidates, option)
		}
	}
	for _, candidate := range []any{field["value"], field["fieldValue"], field["default"]} {
		if !emptyWorkflowValue(candidate) {
			candidates = append(candidates, candidate)
		}
	}
	for _, candidate := range candidates {
		option := optionString(candidate)
		if option == "" {
			continue
		}
		optionCanonical, _ := aspectRatio(option).(string)
		if optionCanonical == canonical {
			return option
		}
	}
	return canonical
}

func ratioParts(value string) ([2]int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return [2]int{}, false
	}
	width, errWidth := strconv.Atoi(parts[0])
	height, errHeight := strconv.Atoi(parts[1])
	return [2]int{width, height}, errWidth == nil && errHeight == nil && width > 0 && height > 0
}

func imageDimensions(value string) ([2]int, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	presets := map[string][2]int{"1:1": {1024, 1024}, "3:2": {1536, 1024}, "2:3": {1024, 1536}, "4:3": {1360, 1024}, "3:4": {1024, 1360}, "4:5": {1024, 1280}, "5:4": {1280, 1024}, "16:9": {1824, 1024}, "9:16": {1024, 1824}, "2:1": {2048, 1024}, "1:2": {1024, 2048}, "21:9": {2352, 1008}, "1:1-2k": {2048, 2048}, "16:9-2k": {2048, 1152}, "9:16-2k": {1152, 2048}, "16:9-4k": {3840, 2160}, "9:16-4k": {2160, 3840}}
	if result, ok := presets[normalized]; ok {
		return result, true
	}
	ratio, ok := aspectRatio(normalized).(string)
	if !ok {
		return [2]int{}, false
	}
	parts, ok := ratioParts(ratio)
	if !ok {
		return [2]int{}, false
	}
	longEdge := 1024
	if strings.HasSuffix(normalized, "-4k") {
		longEdge = 3840
	} else if strings.HasSuffix(normalized, "-2k") {
		longEdge = 2048
	}
	if parts[0] >= parts[1] {
		if longEdge == 1024 {
			return [2]int{roundStep(1024*parts[0]/parts[1], 16), 1024}, true
		}
		return [2]int{longEdge, roundStep(longEdge*parts[1]/parts[0], 16)}, true
	}
	if longEdge == 1024 {
		return [2]int{1024, roundStep(1024*parts[1]/parts[0], 16)}, true
	}
	return [2]int{roundStep(longEdge*parts[0]/parts[1], 16), longEdge}, true
}

func videoDimensions(size, quality string) ([2]int, bool) {
	ratio, ok := aspectRatio(size).(string)
	if !ok {
		return [2]int{}, false
	}
	parts, ok := ratioParts(ratio)
	shortEdge := roundStep(videoResolutionPixels(quality), 16)
	if !ok || shortEdge == 0 {
		return [2]int{}, false
	}
	if parts[0] >= parts[1] {
		return [2]int{roundStep(shortEdge*parts[0]/parts[1], 16), shortEdge}, true
	}
	return [2]int{shortEdge, roundStep(shortEdge*parts[1]/parts[0], 16)}, true
}

func videoResolutionPixels(value any) int {
	normalized := strings.ToLower(strings.Trim(strings.TrimSpace(stringValue(value)), `"`))
	switch normalized {
	case "low":
		return 480
	case "auto", "default", "medium", "high":
		return 720
	case "2k":
		return 1440
	case "4k":
		return 2160
	}
	parsed, _ := strconv.Atoi(strings.TrimSuffix(normalized, "p"))
	return parsed
}

func videoResolutionValue(field jsonMap, value any, size string) any {
	raw := strings.Trim(stringValue(value), `"`)
	if raw == "" {
		return nil
	}
	if isResolutionSelectorField(field, "megapixels") {
		if megapixels := megapixelsForImageShortEdge(size, raw); megapixels != "" {
			return megapixels
		}
		if megapixels := megapixelsForVideoResolution(raw); megapixels != "" {
			return megapixels
		}
		return value
	}
	pixels := videoResolutionPixels(raw)
	if pixels == 0 {
		return value
	}
	candidates := []string{raw, strconv.Itoa(pixels), strconv.Itoa(pixels) + "p"}
	if pixels == 1440 {
		candidates = append(candidates, "2k")
	} else if pixels == 2160 {
		candidates = append(candidates, "4k")
	}
	for _, option := range sliceValue(field["options"]) {
		for _, candidate := range candidates {
			if strings.EqualFold(optionString(option), candidate) {
				return option
			}
		}
	}
	defaultValue := firstValue(field["value"], field["fieldValue"], field["default"])
	if _, ok := defaultValue.(float64); ok {
		return pixels
	}
	normalizedDefault := strings.ToLower(strings.Trim(stringValue(defaultValue), `"`))
	if strings.HasSuffix(normalizedDefault, "p") {
		return strconv.Itoa(pixels) + "p"
	}
	if normalizedDefault == "2k" || normalizedDefault == "4k" {
		if pixels == 1440 {
			return "2k"
		}
		if pixels == 2160 {
			return "4k"
		}
	}
	return value
}

func isResolutionSelectorField(field jsonMap, fieldName string) bool {
	return normalizeSourceName(stringValue(field["classType"])) == "resolutionselector" &&
		normalizeSourceName(stringValue(field["fieldName"])) == normalizeSourceName(fieldName)
}

// MiniMax H3 的 ResolutionSelector 不直接接收 480P/768P，而是用百万像素档位
// 控制输出面积；这里只映射画布使用的通用档位，其他值仍按原样提交。
func megapixelsForVideoResolution(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "480", "480p", "low":
		return "0.4"
	case "768", "768p", "720", "720p":
		return "0.9"
	case "1080", "1080p", "high":
		return "2.0"
	default:
		return ""
	}
}

func megapixelsForImageShortEdge(size, tier string) string {
	shortEdge, ok := imageShortEdgePixels(tier)
	if !ok {
		return ""
	}
	ratio, ok := aspectRatio(size).(string)
	if !ok {
		return ""
	}
	parts, ok := ratioParts(ratio)
	if !ok {
		return ""
	}
	longEdge := float64(shortEdge)
	if parts[0] > parts[1] {
		longEdge = float64(shortEdge) * float64(parts[0]) / float64(parts[1])
	} else if parts[0] < parts[1] {
		longEdge = float64(shortEdge) * float64(parts[1]) / float64(parts[0])
	}
	megapixels := float64(shortEdge) * longEdge / 1048576
	return strconv.FormatFloat(math.Round(megapixels*10)/10, 'f', -1, 64)
}

func imageShortEdgePixels(value string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1k":
		return 1024, true
	case "2k":
		return 2048, true
	case "4k":
		return 4096, true
	default:
		return 0, false
	}
}

var resolutionSelectorAspectRatioOptions = []string{
	"1:1 (Square)",
	"2:3 (Portrait Photo)",
	"3:2 (Photo)",
	"3:4 (Portrait Standard)",
	"4:3 (Standard)",
	"9:16 (Portrait Widescreen)",
	"16:9 (Widescreen)",
	"21:9 (Ultrawide)",
}

func optionString(value any) string {
	if item, ok := mapValue(value); ok {
		for _, key := range []string{"value", "id", "key", "label", "name"} {
			if candidate := strings.Trim(stringValue(item[key]), `"`); candidate != "" {
				return candidate
			}
		}
	}
	return strings.Trim(stringValue(value), `"`)
}

func gcd(left, right int) int {
	for right != 0 {
		left, right = right, left%right
	}
	if left <= 0 {
		return 1
	}
	return left
}

func roundStep(value, step int) int {
	return int(math.Round(float64(value)/float64(step))) * step
}

func stringIn(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func hasAnySuffix(value string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}
