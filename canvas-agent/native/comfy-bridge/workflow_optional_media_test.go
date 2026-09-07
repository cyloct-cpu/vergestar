package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestMultiReferenceTemplateRemovesUnusedLoadImageNodes(t *testing.T) {
	data, err := os.ReadFile(`workflows\H3多参r2v (kj加速).json`)
	if err != nil {
		t.Fatal(err)
	}
	var canvas jsonMap
	if err := json.Unmarshal(data, &canvas); err != nil {
		t.Fatal(err)
	}
	workflow := convertComfyCanvasWorkflow(canvas)
	fields := discoverWorkflowFields(workflow, "video")
	payload := jsonMap{
		"mode":            "video",
		"prompt":          "跳舞",
		"referenceImages": []any{jsonMap{"id": "one"}, jsonMap{"id": "two"}},
		"metadata":        jsonMap{"videoEditOperation": "reference_to_video"},
	}
	payload["workflowFields"] = fields
	if err := validateWorkflowMediaInputs(fields, payload); err != nil {
		t.Fatalf("two reference images should fit the template: %v", err)
	}
	if err := applyWorkflowFields(workflow, payload, map[string]string{"one": "one.png", "two": "two.png"}); err != nil {
		t.Fatalf("apply two reference images: %v", err)
	}

	for _, nodeID := range []string{"164", "165", "166"} {
		if _, exists := workflow[nodeID]; exists {
			t.Fatalf("unused LoadImage %s should be removed", nodeID)
		}
	}
	for _, nodeID := range []string{"146", "148"} {
		if _, exists := workflow[nodeID]; exists {
			t.Fatalf("disabled media consumer %s should be removed after its source is removed", nodeID)
		}
	}
	model, ok := mapValue(workflow["136"])
	if !ok {
		t.Fatal("MiniMaxH3ReferenceToVideo should remain")
	}
	inputs, ok := mapValue(model["inputs"])
	if !ok {
		t.Fatal("MiniMaxH3ReferenceToVideo inputs are missing")
	}
	for _, field := range []string{"ref_images.ref_image_2", "ref_images.ref_image_3", "ref_images.ref_image_4"} {
		if _, exists := inputs[field]; exists {
			t.Fatalf("dangling reference input %s should be removed", field)
		}
	}
	if got := inputs["ref_images.ref_image_0"]; !isWorkflowLink(got) {
		t.Fatalf("first reference input = %#v, want a link", got)
	}
	if got := inputs["ref_images.ref_image_1"]; !isWorkflowLink(got) {
		t.Fatalf("second reference input = %#v, want a link", got)
	}
}

func TestOptionalReferenceVideoIsRemovedWithoutMedia(t *testing.T) {
	workflow := jsonMap{
		"147": jsonMap{
			"class_type": "LoadVideo",
			"inputs":     jsonMap{"file": "sample.mp4"},
		},
		"160": jsonMap{
			"class_type": "Consumer",
			"inputs":     jsonMap{"video": []any{"147", 0}},
		},
		"prompt": jsonMap{
			"class_type": "CLIPTextEncode",
			"inputs":     jsonMap{"text": "sample prompt"},
		},
	}

	fields := discoverWorkflowFields(workflow, "video")
	payload := jsonMap{"mode": "video", "prompt": "跳舞", "referenceVideos": []any{}}
	payload["workflowFields"] = fields
	if err := validateWorkflowMediaInputs(fields, payload); err != nil {
		t.Fatalf("validate without reference videos: %v", err)
	}
	if err := applyWorkflowFields(workflow, payload, nil); err != nil {
		t.Fatalf("apply without reference videos: %v", err)
	}
	removed, _ := removeEmptyMediaLoaderNodes(workflow, fields)
	t.Logf("direct remove = %#v", removed)
	if _, exists := workflow["147"]; exists {
		t.Fatal("unused optional LoadVideo should be removed")
	}
	inputs, ok := mapValue(workflow["160"].(jsonMap)["inputs"])
	if !ok || len(inputs) != 0 {
		t.Fatalf("dependent input should be removed, got %#v", inputs)
	}
}

func TestOptionalReferenceVideoUsesProvidedMedia(t *testing.T) {
	workflow := jsonMap{
		"147": jsonMap{
			"class_type": "LoadVideo",
			"inputs":     jsonMap{"file": "sample.mp4"},
		},
		"prompt": jsonMap{
			"class_type": "CLIPTextEncode",
			"inputs":     jsonMap{"text": "sample prompt"},
		},
	}

	fields := discoverWorkflowFields(workflow, "video")
	payload := jsonMap{
		"mode":            "video",
		"prompt":          "跳舞",
		"referenceVideos": []any{jsonMap{"id": "clip", "name": "clip.mp4"}},
	}
	payload["workflowFields"] = fields
	if err := validateWorkflowMediaInputs(fields, payload); err != nil {
		t.Fatalf("validate with reference video: %v", err)
	}
	if err := applyWorkflowFields(workflow, payload, map[string]string{"clip": "clip.mp4"}); err != nil {
		t.Fatalf("apply with reference video: %v", err)
	}
	if got := stringValue(workflow["147"].(jsonMap)["inputs"].(jsonMap)["file"]); got != "clip.mp4" {
		t.Fatalf("LoadVideo file = %q, want clip.mp4", got)
	}
}

func TestEasyShowAnythingIsCanvasAnnotation(t *testing.T) {
	if !isCanvasAnnotationNode("easy showAnything") {
		t.Fatal("easy showAnything should be excluded from executable API workflow")
	}
}
