package main

import (
	"testing"
)

func TestDiscoverWorkflowFieldsPrefersUserPrompt(t *testing.T) {
	workflow := jsonMap{
		"30:18": jsonMap{
			"class_type": "PrimitiveStringMultiline",
			"inputs":     jsonMap{"value": "You are an expert prompt engineer."},
			"_meta":      jsonMap{"title": "Text String (System Prompt)"},
		},
		"30:19": jsonMap{
			"class_type": "PrimitiveStringMultiline",
			"inputs":     jsonMap{"value": "Ultra low angle action shot"},
			"_meta":      jsonMap{"title": "Text String (User Prompt)"},
		},
	}

	fields := discoverWorkflowFields(workflow, "image")
	var promptField jsonMap
	found := 0
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok || normalizedWorkflowFieldSource(field) != "prompt" {
			continue
		}
		found++
		promptField = field
	}
	if found != 1 {
		t.Fatalf("expected one prompt field, got %d", found)
	}
	if got := stringValue(promptField["nodeId"]); got != "30:19" {
		t.Fatalf("prompt node = %q, want 30:19", got)
	}
	if got := stringValue(promptField["fieldName"]); got != "value" {
		t.Fatalf("prompt field = %q, want value", got)
	}
}

func TestApplyPromptFallbackUsesUserPromptTarget(t *testing.T) {
	workflow := jsonMap{
		"30:18": jsonMap{
			"class_type": "PrimitiveStringMultiline",
			"inputs":     jsonMap{"value": "You are an expert prompt engineer."},
			"_meta":      jsonMap{"title": "Text String (System Prompt)"},
		},
		"30:19": jsonMap{
			"class_type": "PrimitiveStringMultiline",
			"inputs":     jsonMap{"value": "Ultra low angle action shot"},
			"_meta":      jsonMap{"title": "Text String (User Prompt)"},
		},
	}

	applyPromptFallback(workflow, "一个美国女孩在自拍", []any{})
	if got := stringValue(workflow["30:18"].(jsonMap)["inputs"].(jsonMap)["value"]); got != "You are an expert prompt engineer." {
		t.Fatalf("system prompt changed: %q", got)
	}
	if got := stringValue(workflow["30:19"].(jsonMap)["inputs"].(jsonMap)["value"]); got != "一个美国女孩在自拍" {
		t.Fatalf("user prompt = %q, want replacement", got)
	}
}

func TestDiscoverWorkflowFieldsRecognizesResolutionSelector(t *testing.T) {
	workflow := jsonMap{
		"115": jsonMap{
			"class_type": "ResolutionSelector",
			"inputs": jsonMap{
				"aspect_ratio": "16:9 (Widescreen)",
				"megapixels":   0.4,
			},
		},
	}

	fields := discoverWorkflowFields(workflow, "video")
	sources := map[string]string{}
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok {
			continue
		}
		sources[stringValue(field["fieldName"])] = normalizedWorkflowFieldSource(field)
	}
	if sources["aspect_ratio"] != "aspectratio" {
		t.Fatalf("aspect_ratio source = %q, want aspectratio", sources["aspect_ratio"])
	}
	if sources["megapixels"] != "vquality" {
		t.Fatalf("megapixels source = %q, want vquality", sources["megapixels"])
	}
}

func TestVideoResolutionValueMapsResolutionSelectorMegapixels(t *testing.T) {
	field := jsonMap{"classType": "ResolutionSelector", "fieldName": "megapixels", "fieldValue": 0.4}
	if got := videoResolutionValue(field, "480P", ""); got != "0.4" {
		t.Fatalf("480P value = %v, want 0.4", got)
	}
	if got := videoResolutionValue(field, "768P", ""); got != "0.9" {
		t.Fatalf("768P value = %v, want 0.9", got)
	}
	if got := videoResolutionValue(field, "1080P", ""); got != "2.0" {
		t.Fatalf("1080P value = %v, want 2.0", got)
	}
}

func TestDiscoverWorkflowFieldsRecognizesDurationNodeTitle(t *testing.T) {
	workflow := jsonMap{
		"105:111": jsonMap{
			"class_type": "PrimitiveFloat",
			"_meta":      jsonMap{"title": "Float (duration)"},
			"inputs":     jsonMap{"value": 5},
		},
	}

	fields := discoverWorkflowFields(workflow, "video")
	durationFields := make([]jsonMap, 0, 1)
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok || stringValue(field["nodeId"]) != "105:111" {
			continue
		}
		durationFields = append(durationFields, field)
	}
	if len(durationFields) != 1 {
		t.Fatalf("expected one duration field, got %d", len(durationFields))
	}
	field := durationFields[0]
	if got := stringValue(field["fieldType"]); got != "DURATION" {
		t.Fatalf("fieldType = %q, want DURATION", got)
	}
	if got := normalizedWorkflowFieldSource(field); got != "videoseconds" {
		t.Fatalf("source = %q, want videoseconds", got)
	}
	if field["min"] != 4 {
		t.Fatalf("min = %v, want 4", field["min"])
	}
	if field["max"] != 15 {
		t.Fatalf("max = %v, want 15", field["max"])
	}
	if field["step"] != 1 {
		t.Fatalf("step = %v, want 1", field["step"])
	}
}

func TestImageResolutionValueUsesShortEdgeMegapixels(t *testing.T) {
	field := jsonMap{"classType": "ResolutionSelector", "fieldName": "megapixels", "fieldValue": 1.0}
	cases := []struct {
		size string
		tier string
		want string
	}{
		{size: "1536x1024", tier: "1K", want: "1.5"},
		{size: "1:1", tier: "2K", want: "4"},
		{size: "2:3 (Portrait Photo)", tier: "2K", want: "6"},
	}
	for _, item := range cases {
		if got := videoResolutionValue(field, item.tier, item.size); got != item.want {
			t.Fatalf("%s %s value = %v, want %s", item.size, item.tier, got, item.want)
		}
	}
}

func TestImageShortEdgeMegapixels(t *testing.T) {
	if got := megapixelsForImageShortEdge("1536x1024", "1K"); got != "1.5" {
		t.Fatalf("3:2 1K megapixels = %s, want 1.5", got)
	}
	if got := megapixelsForImageShortEdge("1:1", "2K"); got != "4" {
		t.Fatalf("1:1 2K megapixels = %s, want 4", got)
	}
}

func TestWorkflowImageFieldsDiscoverFrameRoles(t *testing.T) {
	workflow := jsonMap{
		"10": jsonMap{
			"class_type": "LoadImage",
			"_meta":      jsonMap{"title": "首帧"},
			"inputs":     jsonMap{"image": "start.png"},
		},
		"11": jsonMap{
			"class_type": "LoadImage",
			"_meta":      jsonMap{"title": "尾帧"},
			"inputs":     jsonMap{"image": "end.png"},
		},
	}

	fields := discoverWorkflowFields(workflow, "video")
	roles := []string{}
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok || normalizedWorkflowFieldSource(field) != "referenceimage" {
			continue
		}
		roles = append(roles, stringValue(field["role"]))
	}
	if len(roles) != 2 || roles[0] != "first_frame" || roles[1] != "last_frame" {
		t.Fatalf("frame roles = %v, want [first_frame last_frame]", roles)
	}
}

func TestResolveSourceUsesExplicitFrameRoles(t *testing.T) {
	payload := jsonMap{
		"referenceImages": []any{
			jsonMap{"id": "start", "name": "start.png"},
			jsonMap{"id": "end", "name": "end.png"},
			jsonMap{"id": "style", "name": "style.png"},
		},
		"metadata": jsonMap{"videoStartFrameNodeId": "start", "videoEndFrameNodeId": "end"},
	}
	files := map[string]string{"start": "start.png", "end": "end.png", "style": "style.png"}
	fields := []jsonMap{
		{"source": "referenceImage", "role": "first_frame"},
		{"source": "referenceImage", "role": "last_frame"},
		{"source": "referenceImage", "role": "reference_image"},
	}
	want := []string{"start.png", "end.png", "style.png"}
	allocation := newReferenceImageAllocation(payload)
	for i, field := range fields {
		if got := resolveSource("referenceImage", field, payload, files, allocation); got != want[i] {
			t.Fatalf("field %d = %v, want %v", i, got, want[i])
		}
	}
}

func TestResolveSourceAllocatesPlainReferenceImagesOnce(t *testing.T) {
	payload := jsonMap{
		"referenceImages": []any{
			jsonMap{"id": "one", "name": "one.png"},
			jsonMap{"id": "two", "name": "two.png"},
		},
		"workflowFields": []any{
			jsonMap{"source": "referenceImage"},
			jsonMap{"source": "referenceImage"},
		},
	}
	files := map[string]string{"one": "one.png", "two": "two.png"}
	allocation := newReferenceImageAllocation(payload)

	if got := resolveSource("referenceImage", jsonMap{"source": "referenceImage"}, payload, files, allocation); got != "one.png" {
		t.Fatalf("first image = %v, want one.png", got)
	}
	if got := resolveSource("referenceImage", jsonMap{"source": "referenceImage"}, payload, files, allocation); got != "two.png" {
		t.Fatalf("second image = %v, want two.png", got)
	}
	if got := resolveSource("referenceImage", jsonMap{"source": "referenceImage"}, payload, files, allocation); got != nil {
		t.Fatalf("exhausted image = %v, want nil", got)
	}
}

func TestResolveSourceAllocatesMixedFrameAndReferenceImages(t *testing.T) {
	payload := jsonMap{
		"referenceImages": []any{
			jsonMap{"id": "start", "name": "start.png"},
			jsonMap{"id": "end", "name": "end.png"},
			jsonMap{"id": "style", "name": "style.png"},
		},
		"metadata": jsonMap{"videoStartFrameNodeId": "start", "videoEndFrameNodeId": "end"},
		"workflowFields": []any{
			jsonMap{"source": "referenceImage", "role": "first_frame"},
			jsonMap{"source": "referenceImage", "role": "last_frame"},
			jsonMap{"source": "referenceImage", "role": "reference_image"},
		},
	}
	files := map[string]string{"start": "start.png", "end": "end.png", "style": "style.png"}
	allocation := newReferenceImageAllocation(payload)

	if got := resolveSource("referenceImage", jsonMap{"source": "referenceImage", "role": "reference_image"}, payload, files, allocation); got != "style.png" {
		t.Fatalf("ordinary reference = %v, want style.png", got)
	}
	if got := resolveSource("referenceImage", jsonMap{"source": "referenceImage", "role": "first_frame"}, payload, files, allocation); got != "start.png" {
		t.Fatalf("first frame = %v, want start.png", got)
	}
	if got := resolveSource("referenceImage", jsonMap{"source": "referenceImage", "role": "last_frame"}, payload, files, allocation); got != "end.png" {
		t.Fatalf("last frame = %v, want end.png", got)
	}
	if got := resolveSource("referenceImage", jsonMap{"source": "referenceImage", "role": "reference_image"}, payload, files, allocation); got != nil {
		t.Fatalf("second ordinary reference = %v, want nil", got)
	}
}

func TestResolveSourceAllocatesGenericFieldsWithFrameMetadata(t *testing.T) {
	payload := jsonMap{
		"referenceImages": []any{
			jsonMap{"id": "start", "name": "start.png"},
			jsonMap{"id": "end", "name": "end.png"},
		},
		"metadata": jsonMap{"videoStartFrameNodeId": "start", "videoEndFrameNodeId": "end"},
		"workflowFields": []any{
			jsonMap{"source": "referenceImage", "role": "media"},
			jsonMap{"source": "referenceImage", "role": "media"},
		},
	}
	files := map[string]string{"start": "start.png", "end": "end.png"}
	allocation := newReferenceImageAllocation(payload)

	if got := resolveSource("referenceImage", jsonMap{"source": "referenceImage", "role": "media"}, payload, files, allocation); got != "start.png" {
		t.Fatalf("first generic field = %v, want start.png", got)
	}
	if got := resolveSource("referenceImage", jsonMap{"source": "referenceImage", "role": "media"}, payload, files, allocation); got != "end.png" {
		t.Fatalf("second generic field = %v, want end.png", got)
	}
}

func TestWorkflowImageBranchesFollowVideoModelLinks(t *testing.T) {
	workflow := jsonMap{
		"10": jsonMap{
			"class_type": "LoadImage",
			"inputs":     jsonMap{"image": "start.png"},
		},
		"20": jsonMap{
			"class_type": "MiniMaxH3ImageToVideo",
			"inputs":     jsonMap{"start_image": []any{"10", 0}},
		},
		"30": jsonMap{
			"class_type": "LoadImage",
			"inputs":     jsonMap{"image": "reference.png"},
		},
		"40": jsonMap{
			"class_type": "MiniMaxH3ReferenceToVideo",
			"inputs":     jsonMap{"ref_image_1": []any{"30", 0}},
		},
	}

	fields := discoverWorkflowFields(workflow, "video")
	branches := map[string]string{}
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok || normalizedWorkflowFieldSource(field) != "referenceimage" {
			continue
		}
		branches[stringValue(field["nodeId"])] = stringValue(field["mediaBranch"])
	}
	if branches["10"] != "frames" || branches["30"] != "reference" {
		t.Fatalf("image branches = %v, want 10=frames and 30=reference", branches)
	}
}

func TestReferenceAllocationPrefersDirectReferenceBranch(t *testing.T) {
	payload := jsonMap{
		"referenceImages": []any{
			jsonMap{"id": "first", "name": "first.png"},
			jsonMap{"id": "second", "name": "second.png"},
		},
		"metadata":       jsonMap{"videoEditOperation": "reference_to_video"},
		"workflowFields": []any{},
	}
	files := map[string]string{"first": "first.png", "second": "second.png"}
	allocation := newReferenceImageAllocation(payload)

	framesField := jsonMap{"source": "referenceImage", "mediaBranch": "frames", "role": "first_frame", "classType": "LoadImage", "fieldName": "image"}
	referenceField := jsonMap{"source": "referenceImage", "mediaBranch": "reference", "classType": "MiniMaxH3ReferenceToVideo", "fieldName": "ref_image_1"}
	indirectField := jsonMap{"source": "referenceImage", "mediaBranch": "reference", "classType": "ImageResize", "fieldName": "image"}

	if got := resolveSource("referenceImage", framesField, payload, files, allocation); got != nil {
		t.Fatalf("frames field consumed image: %v, want nil", got)
	}
	if got := resolveSource("referenceImage", referenceField, payload, files, allocation); got != "first.png" {
		t.Fatalf("direct reference = %v, want first.png", got)
	}
	if got := resolveSource("referenceImage", indirectField, payload, files, allocation); got != "second.png" {
		t.Fatalf("indirect reference = %v, want second.png", got)
	}
}

func TestValidateWorkflowMediaInputsUsesReferenceBranchCapacity(t *testing.T) {
	fields := []any{
		jsonMap{"source": "referenceImage", "mediaBranch": "frames", "role": "first_frame", "sourceIndex": 0},
		jsonMap{"source": "referenceImage", "mediaBranch": "frames", "role": "last_frame", "sourceIndex": 1},
		jsonMap{"source": "referenceImage", "mediaBranch": "reference", "classType": "MiniMaxH3ReferenceToVideo", "fieldName": "ref_image_1", "sourceIndex": 0},
		jsonMap{"source": "referenceImage", "mediaBranch": "reference", "classType": "LoadImage", "fieldName": "image", "sourceIndex": 2},
		jsonMap{"source": "referenceImage", "mediaBranch": "reference", "classType": "MiniMaxH3ReferenceToVideo", "fieldName": "ref_image_2", "sourceIndex": 1},
	}
	payload := jsonMap{
		"referenceImages": []any{jsonMap{"id": "one"}, jsonMap{"id": "two"}},
		"metadata":        jsonMap{"videoEditOperation": "reference_to_video"},
		"workflowFields":  fields,
	}

	if err := validateWorkflowMediaInputs(fields, payload); err != nil {
		t.Fatalf("two direct reference slots should be valid: %v", err)
	}
	payload["referenceImages"] = append(payload["referenceImages"].([]any), jsonMap{"id": "three"})
	t.Logf("reference capacity = %d", effectiveReferenceImageCapacity(fields, payload))
	if err := validateWorkflowMediaInputs(fields, payload); err == nil {
		t.Fatal("three images should exceed two direct reference slots")
	}
}

func TestDiscoverWorkflowFieldsBindsDirectModelPrompt(t *testing.T) {
	workflow := jsonMap{
		"10": jsonMap{
			"class_type": "CLIPTextEncode",
			"inputs":     jsonMap{"text": "ignored positive prompt"},
		},
		"20": jsonMap{
			"class_type": "MiniMaxH3ReferenceToVideo",
			"inputs":     jsonMap{"prompt": "内置模型提示词"},
		},
		"30": jsonMap{
			"class_type": "MiniMaxH3ImageToVideo",
			"inputs":     jsonMap{"prompt": "另一个分支提示词"},
		},
	}

	fields := discoverWorkflowFields(workflow, "video")
	prompts := map[string]bool{}
	for _, raw := range fields {
		field, ok := mapValue(raw)
		if !ok || normalizedWorkflowFieldSource(field) != "prompt" {
			continue
		}
		prompts[stringValue(field["nodeId"])+"::"+stringValue(field["fieldName"])] = true
	}
	if len(prompts) != 2 || !prompts["20::prompt"] || !prompts["30::prompt"] {
		t.Fatalf("prompt fields = %v, want direct model prompts only", prompts)
	}
}

func TestVideoDimensionsUseSixteenPixelSteps(t *testing.T) {
	cases := []struct {
		ratio   string
		quality string
		width   int
		height  int
	}{
		{ratio: "16:9", quality: "480P", width: 848, height: 480},
		{ratio: "16:9", quality: "720P", width: 1280, height: 720},
		{ratio: "16:9", quality: "1080P", width: 1936, height: 1088},
	}
	for _, item := range cases {
		dimensions, ok := videoDimensions(item.ratio, item.quality)
		if !ok {
			t.Fatalf("videoDimensions(%q, %q) failed", item.ratio, item.quality)
		}
		width, height := dimensions[0], dimensions[1]
		if width != item.width || height != item.height {
			t.Fatalf("videoDimensions(%q, %q) = %dx%d, want %dx%d", item.ratio, item.quality, width, height, item.width, item.height)
		}
	}
}

func TestImageDimensionsUseSixteenPixelSteps(t *testing.T) {
	dimensions, ok := imageDimensions("21:9")
	if !ok || dimensions[0] != 2352 || dimensions[1] != 1008 {
		t.Fatalf("imageDimensions(21:9) = %dx%d, %v, want 2352x1008, true", dimensions[0], dimensions[1], ok)
	}
	dimensions, ok = imageDimensions("7:3")
	if !ok || dimensions[0] != 2384 || dimensions[1] != 1024 {
		t.Fatalf("imageDimensions(7:3) = %dx%d, %v, want 2384x1024, true", dimensions[0], dimensions[1], ok)
	}
	dimensions, ok = pixelDimensions("1000x1000")
	if !ok {
		t.Fatal("pixelDimensions(1000x1000) failed")
	}
	width, height := dimensions[0], dimensions[1]
	if roundStep(width, 16) != 1008 || roundStep(height, 16) != 1008 {
		t.Fatalf("custom 1000x1000 rounding = %dx%d, want 1008x1008", roundStep(width, 16), roundStep(height, 16))
	}
}
