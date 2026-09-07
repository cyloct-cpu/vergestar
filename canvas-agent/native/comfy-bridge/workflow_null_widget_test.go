package main

import "testing"

func TestConvertComfyCanvasWorkflowSkipsNullWidgetValues(t *testing.T) {
	workflow := jsonMap{
		"nodes": []any{
			jsonMap{
				"id":         "168",
				"type":       "PrimitiveNode",
				"properties": jsonMap{},
				"inputs": []any{
					jsonMap{"name": "model", "type": "MODEL", "link": 1},
					jsonMap{"name": "clip", "type": "CLIP", "link": nil},
					jsonMap{"name": "lora_name", "type": "COMBO", "link": nil, "widget": jsonMap{"name": "lora_name"}},
					jsonMap{"name": "strength_model", "type": "FLOAT", "link": nil, "widget": jsonMap{"name": "strength_model"}},
				},
				"widgets_values": []any{nil, 1},
			},
		},
		"links": []any{
			[]any{1, "100", 0, "168", 0, "MODEL"},
		},
	}

	converted := convertComfyCanvasWorkflow(workflow)
	node, ok := converted["168"].(jsonMap)
	if !ok {
		t.Fatal("LoraLoader node was not converted")
	}
	inputs, ok := node["inputs"].(jsonMap)
	if !ok {
		t.Fatal("converted inputs are missing")
	}
	if _, exists := inputs["lora_name"]; exists {
		t.Fatal("null lora_name should be omitted")
	}
}

func TestConvertComfyCanvasWorkflowBypassesUnconfiguredLora(t *testing.T) {
	workflow := jsonMap{
		"nodes": []any{
			jsonMap{"id": "100", "type": "UNETLoader", "properties": jsonMap{}, "inputs": []any{
				jsonMap{"name": "unet_name", "type": "COMBO", "link": nil, "widget": jsonMap{"name": "unet_name"}},
			}, "widgets_values": []any{"base.safetensors"}},
			jsonMap{"id": "168", "type": "LoraLoader", "properties": jsonMap{}, "inputs": []any{
				jsonMap{"name": "model", "type": "MODEL", "link": 1},
				jsonMap{"name": "lora_name", "type": "COMBO", "link": nil, "widget": jsonMap{"name": "lora_name"}},
			}, "widgets_values": []any{nil}},
			jsonMap{"id": "126", "type": "BasicGuider", "properties": jsonMap{}, "inputs": []any{
				jsonMap{"name": "model", "type": "MODEL", "link": 2},
			}},
		},
		"links": []any{
			[]any{1, "100", 0, "168", 0, "MODEL"},
			[]any{2, "168", 0, "126", 0, "MODEL"},
		},
	}

	converted := convertComfyCanvasWorkflow(workflow)
	if _, exists := converted["168"]; exists {
		t.Fatal("unconfigured LoRA should be bypassed")
	}
	guider, ok := converted["126"].(jsonMap)
	if !ok {
		t.Fatal("downstream node was not converted")
	}
	guiderInputs := guider["inputs"].(jsonMap)
	link := guiderInputs["model"].([]any)
	if link[0] != "100" || link[1] != 0 {
		t.Fatalf("model link = %v, want 100:0", link)
	}
}
