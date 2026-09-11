import assert from "node:assert/strict";
import test from "node:test";

import { listNovelSkills, resolveNovelSkills } from "../dist/skills.js";

test("loads vendored InkOS skills and reports unresolved ids", async () => {
    const skills = await listNovelSkills();
    assert.ok(skills.some((skill) => skill.id === "inkos-long-writing"));

    const resolved = await resolveNovelSkills(["inkos-long-writing", "missing-skill"]);
    assert.equal(resolved.skills[0]?.id, "inkos-long-writing");
    assert.deepEqual(resolved.missingSkillIds, ["missing-skill"]);
});
