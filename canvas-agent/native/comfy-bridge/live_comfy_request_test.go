package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"
)

func workflowPathForTest(workflowID string) string {
	return "workflows/" + workflowID
}

func jsonUnmarshalForTest(data []byte, target *jsonMap) error {
	return json.Unmarshal(data, target)
}

func fetchHistoryForTest(comfy, promptID string) (jsonMap, error) {
	var payload jsonMap
	if err := requestComfyJSON("GET", comfy+"/history/"+url.PathEscape(promptID), nil, &payload); err != nil {
		return nil, err
	}
	history, ok := mapValue(payload[promptID])
	if !ok {
		return nil, errors.New("history entry not found")
	}
	return history, nil
}

func TestLiveComfyRequestScenarios(t *testing.T) {
	if os.Getenv("RUN_COMFY_LIVE") != "1" {
		t.Skip("set RUN_COMFY_LIVE=1 to run the protected ComfyUI integration test")
	}

	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData)
	one := jsonMap{"id": "live-image-one", "name": "live-one.png", "dataUrl": dataURL, "mimeType": "image/png"}
	two := jsonMap{"id": "live-image-two", "name": "live-two.png", "dataUrl": dataURL, "mimeType": "image/png"}

	cases := []struct {
		name       string
		workflowID string
		mode       string
		prompt     string
		references []any
		metadata   jsonMap
	}{
		{name: "text to image", workflowID: "image_krea2_turbo_t2i_int8.json", mode: "image", prompt: "a red apple on a white table, live integration test"},
		{name: "start end to video", workflowID: "H3文生图_首尾帧.json", mode: "video", prompt: "the character walks slowly forward, live integration test", references: []any{one, two}, metadata: jsonMap{"videoEditOperation": "image_to_video"}},
		{name: "multi reference video", workflowID: "H3多参r2v (kj加速).json", mode: "video", prompt: "<Picture 1> and <Picture 2> dance together, live integration test", references: []any{one, two}, metadata: jsonMap{"videoEditOperation": "reference_to_video"}},
	}

	for _, item := range cases {
		item := item
		t.Run(item.name, func(t *testing.T) {
			data, err := os.ReadFile(workflowPathForTest(item.workflowID))
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
			fields := discoverWorkflowFields(workflow, item.mode)
			payload := jsonMap{
				"mode":           item.mode,
				"prompt":         item.prompt,
				"metadata":       item.metadata,
				"workflowId":     item.workflowID,
				"workflowFields": fields,
			}
			if item.references != nil {
				payload["referenceImages"] = item.references
			}

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
			applyPromptFallback(workflow, item.prompt, fields)
			stripCanvasAnnotationNodes(workflow)

			if item.name == "multi reference video" {
				for _, removed := range []string{"164", "165", "166"} {
					if _, exists := workflow[removed]; exists {
						t.Fatalf("empty media loader %s should be removed", removed)
					}
				}
			}

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
						outputs, _ := mapValue(history["outputs"])
						if len(outputs) == 0 {
							t.Fatal("workflow completed without outputs")
						}
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
