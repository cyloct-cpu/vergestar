const fs = require("fs");
const p = "internal/service/novel_agent.go";
let t = fs.readFileSync(p, "utf8").replace(/\r\n/g, "\n");

// StartNovelAgentJob：接受 job 后立即持久化 user 消息（session 不存在则创建）。
const startAnchor = "\tvar body struct {\n\t\tJob NovelAgentJob `json:\"job\"`\n\t}\n\tif err := json.NewDecoder(resp.Body).Decode(&body); err != nil {\n\t\treturn NovelAgentJob{}, WrapAppError(http.StatusBadGateway, \"小说 Agent 任务响应无效\", err)\n\t}\n\tif strings.TrimSpace(request.ConfirmedIntent) != \"\" {";
if (!t.includes(startAnchor)) { console.error("start anchor missing"); process.exit(1); }
const startNew = [
  "\tvar body struct {",
  "\t\tJob NovelAgentJob `json:\"job\"`",
  "\t}",
  "\tif err := json.NewDecoder(resp.Body).Decode(&body); err != nil {",
  "\t\treturn NovelAgentJob{}, WrapAppError(http.StatusBadGateway, \"小说 Agent 任务响应无效\", err)",
  "\t}",
  "\t// 任务发起即持久化用户消息与会话行：确认动作的结果/确认卡在轮询终态时补挂，刷新不丢。",
  "\tif err := s.ensureNovelTurnUserMessage(userID, request); err != nil {",
  "\t\tlog.Printf(\"novel turn user message persist failed: user=%s err=%v\", userID, err)",
  "\t}",
  "\tif strings.TrimSpace(request.ConfirmedIntent) != \"\" {",
].join("\n");
t = t.replace(startAnchor, startNew);

// 轮询 succeeded 时补 assistant 消息（幂等：ID=jobID）。
const pollAnchor = "\t\ts.mirrorNovelAgentJob(userID, body.Job, NovelAgentJobRequestMeta{SessionID: sessionIDOf(body.Job)})\n\t\treturn body.Job, nil\n\t}";
if (!t.includes(pollAnchor)) { console.error("poll anchor missing"); process.exit(1); }
const pollNew = [
  "\t\ts.mirrorNovelAgentJob(userID, body.Job, NovelAgentJobRequestMeta{SessionID: sessionIDOf(body.Job)})",
  "\t\ts.persistNovelJobAssistantMessage(userID, body.Job)",
  "\t\treturn body.Job, nil",
  "\t}",
].join("\n");
// 该锚点带缩进层级不同：先确认逐字存在（带前导 tab）。
const pollAnchorTabbed = pollAnchor.replace(/\n/g, "\n");
if (!t.includes(pollAnchorTabbed)) { console.error("poll anchor (tabbed) missing"); process.exit(1); }
t = t.replace(pollAnchorTabbed, pollNew);

fs.writeFileSync(p, t);
console.log("persist hooks wired");
