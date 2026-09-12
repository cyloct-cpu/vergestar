package app

import (
	"log"
	"strings"
)

// backfillNovelShotRevisionFields fills empty structured fields on the current
// revision from the storyboard table row. Only writes when at least one field
// is empty and the row carries data; never overwrites non-empty values.
func (s *Service) backfillNovelShotRevisionFields(revisionID string, row novelStoryboardRow) {
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
