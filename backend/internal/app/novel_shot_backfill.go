package app

import (
	"log"
	"strings"
)

// backfillNovelShotRevisionFields fills empty structured fields on the current
// revision from the storyboard table row. Only writes when at least one field
// is empty and the row carries data; never overwrites non-empty values.
func (s *Service) backfillNovelShotRevisionFields(revisionID, shotID string, row novelStoryboardRow) {
	if revisionID == "" {
		return
	}
	revision, err := s.repo.ShotRevisionByID(revisionID)
	if err != nil || revision == nil {
		return
	}
	shotSize := revision.ShotSize
	if shotSize == "" {
		shotSize = novelShotSizeFrom(row.ShotSize)
	}
	cameraAngle := revision.CameraAngle
	if cameraAngle == "" {
		cameraAngle = novelCameraAngleFrom(row.CameraAngle)
	}
	dialogue := revision.Dialogue
	if dialogue == "" && row.Dialogue != "" {
		dialogue = row.Dialogue
	}
	action := revision.Action
	if action == "" && row.Action != "" {
		action = row.Action
	}
	// Shot 表 durationMs 回填——与 revision 字段变化无关，每次检查（首轮可能已填
	// 字段导致早退跳过 duration）。
	if row.DurationMs > 0 && shotID != "" {
		if err := s.repo.UpdateShotDurationIfZero(shotID, row.DurationMs); err != nil {
			log.Printf("novel shot duration backfill failed: shot=%s err=%v", shotID, err)
		} else {
			log.Printf("novel shot duration backfilled: shot=%s dur=%d", shotID, row.DurationMs)
		}
	}
	if shotSize == revision.ShotSize && cameraAngle == revision.CameraAngle && dialogue == revision.Dialogue && action == revision.Action {
		return
	}
	revision.ShotSize = shotSize
	revision.CameraAngle = cameraAngle
	revision.Dialogue = dialogue
	revision.Action = action
	if err := s.repo.UpdateShotRevisionFields(revision); err != nil {
		log.Printf("novel shot revision backfill failed: revision=%s err=%v", revisionID, err)
	}
}

var _ = strings.TrimSpace
