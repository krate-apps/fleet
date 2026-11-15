package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/krate-apps/fleet/internal/jobs"
)

// StartMktorrentJob creates a tools.mktorrent job and runs CreateTorrent asynchronously.
// extensionID may be empty for HTTP-started jobs; workers that poll via fleet.jobs.get
// must pass their own extension id so ownership checks succeed.
func StartMktorrentJob(jm *jobs.Manager, template *Runner, dataDir, extensionID string, in CreateInput) (*jobs.Job, error) {
	if jm == nil {
		return nil, fmt.Errorf("jobs unavailable")
	}
	if template == nil {
		return nil, fmt.Errorf("tools runner unavailable")
	}
	job, err := jm.Create(jobs.CreateInput{
		Kind:        "tools.mktorrent",
		ExtensionID: extensionID,
		Payload:     in,
	})
	if err != nil {
		return nil, err
	}
	go RunMktorrentJob(jm, template, dataDir, job.ID, in)
	return job, nil
}

// RunMktorrentJob executes mktorrent for an existing job id (used by StartMktorrentJob).
func RunMktorrentJob(jm *jobs.Manager, template *Runner, dataDir, jobID string, in CreateInput) {
	if jm == nil || template == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	_ = jm.MarkRunning(jobID, "creating torrent")

	artifactDir := filepath.Join(dataDir, "artifacts", jobID)
	runner := &Runner{
		MktorrentPath:   template.MktorrentPath,
		DumptorrentPath: template.DumptorrentPath,
		AllowedRoots:    append([]string(nil), template.AllowedRoots...),
		ArtifactsDir:    artifactDir,
	}
	res, err := runner.CreateTorrent(ctx, in)
	if err != nil {
		_ = jm.Fail(jobID, err.Error())
		return
	}
	_ = jm.Succeed(jobID, map[string]any{
		"outputPath":    res.OutputPath,
		"downloadPath":  "/api/v1/jobs/" + jobID + "/download",
		"torrentBase64": res.TorrentBase64,
		"meta":          res.Meta,
		"jsonCli":       res.JSONCLI,
	})
}
