package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func durationWorkflowCases() []struct {
	name       string
	workflowID string
	nodeID     string
	mode       string
	operation  string
	refs       int
} {
	return []struct {
		name       string
		workflowID string
		nodeID     string
		mode       string
		operation  string
		refs       int
	}{
		{name: "reference to video", workflowID: "h3参考生视频.json", nodeID: "132", mode: "video", operation: "reference_to_video", refs: 1},
		{name: "multi reference video", workflowID: "H3多参r2v (kj加速).json", nodeID: "132", mode: "video", operation: "reference_to_video", refs: 2},
		{name: "start end video", workflowID: "H3文生图_首尾帧.json", nodeID: "105:111", mode: "video", operation: "image_to_video", refs: 2},
	}
}

func TestApplyDurationUpdatesWorkflowValue(t *testing.T) {
	for _, item := range durationWorkflowCases() {
		t.Run(item.name, func(t *testing.T) {
			workflow, fields := executableWorkflowForFile(t, item.workflowID, item.mode)
			payload := durationTestPayload(item.mode, item.operation, item.refs, 8)
			payload["workflowFields"] = fields
			files := durationTestFiles(t, item.refs)
			if err := applyWorkflowFields(workflow, payload, files); err != nil {
				t.Fatalf("apply workflow fields: %v", err)
			}
			node, ok := workflow[item.nodeID]
			if !ok {
				t.Fatalf("duration node %s missing", item.nodeID)
			}
			nodeMap, ok := mapValue(node)
			if !ok {
				t.Fatalf("duration node %s is not an object", item.nodeID)
			}
			inputs, _ := mapValue(nodeMap["inputs"])
			if got := numberValue(inputs["value"]); got != 8 {
				t.Fatalf("duration value = %v, want 8", inputs["value"])
			}
		})
	}
}

func TestLiveDurationVideoGeneration(t *testing.T) {
	if os.Getenv("RUN_COMFY_LIVE") != "1" {
		t.Skip("set RUN_COMFY_LIVE=1 to run the protected ComfyUI integration test")
	}
	caseIndex := os.Getenv("RUN_COMFY_LIVE_CASE")
	for _, item := range durationWorkflowCases() {
		if caseIndex != "" && caseIndex != item.name {
			continue
		}
		t.Run(item.name, func(t *testing.T) {
			workflow, fields := executableWorkflowForFile(t, item.workflowID, item.mode)
			payload := durationTestPayload(item.mode, item.operation, item.refs, 8)
			payload["workflowFields"] = fields
			if err := validateWorkflowMediaInputs(fields, payload); err != nil {
				t.Fatalf("validate media inputs: %v", err)
			}
			files, err := uploadReferences("http://127.0.0.1:8188", payload)
			if err != nil {
				t.Fatalf("upload references: %v", err)
			}
			if err := applyWorkflowFields(workflow, payload, files); err != nil {
				t.Fatalf("apply workflow fields: %v", err)
			}
			applyPromptFallback(workflow, "跳舞", fields)
			stripCanvasAnnotationNodes(workflow)
			promptID, err := submitPrompt("http://127.0.0.1:8188", workflow)
			if err != nil {
				t.Fatalf("submit prompt: %v", err)
			}
			deadline := time.Now().Add(60 * time.Minute)
			for time.Now().Before(deadline) {
				history, err := fetchHistoryForTest("http://127.0.0.1:8188", promptID)
				if err == nil {
					completed, completionErr := completedHistory(history)
					if completionErr != nil {
						t.Fatalf("workflow failed: %v", completionErr)
					}
					if completed {
						t.Logf("workflow completed: prompt_id=%s", promptID)
						return
					}
				}
				time.Sleep(2 * time.Second)
			}
			t.Fatalf("workflow timed out: prompt_id=%s", promptID)
		})
	}
}

func executableWorkflowForFile(t *testing.T, workflowID, mode string) (jsonMap, []any) {
	t.Helper()
	data, err := os.ReadFile("workflows/" + workflowID)
	if err != nil {
		t.Fatal(err)
	}
	var source jsonMap
	if err := jsonUnmarshalForTest(data, &source); err != nil {
		t.Fatal(err)
	}
	workflow, err := executableWorkflow(source)
	if err != nil {
		t.Fatal(err)
	}
	return workflow, discoverWorkflowFields(workflow, mode)
}

func durationTestPayload(mode, operation string, refs int, seconds float64) jsonMap {
	pngData, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData)
	images := make([]any, 0, refs)
	for index := 0; index < refs; index++ {
		images = append(images, jsonMap{"id": "duration-test-" + string(rune('a'+index)), "name": "duration-test.png", "dataUrl": dataURL, "mimeType": "image/png"})
	}
	payload := jsonMap{
		"mode":   mode,
		"prompt": "跳舞",
		"metadata": jsonMap{
			"videoEditOperation": operation,
		},
		"params": jsonMap{
			"videoSeconds": seconds,
			"vquality":     "768p",
			"size":         "1366x768",
		},
	}
	if refs > 0 {
		payload["referenceImages"] = images
	}
	return payload
}

func durationTestFiles(t *testing.T, refs int) map[string]string {
	t.Helper()
	pngData, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	files := map[string]string{}
	for index := 0; index < refs; index++ {
		id := "duration-test-" + string(rune('a'+index))
		path := filepath.Join(t.TempDir(), id+".png")
		if err := os.WriteFile(path, pngData, 0o644); err != nil {
			t.Fatal(err)
		}
		files[id] = path
	}
	return files
}
