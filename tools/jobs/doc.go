// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package jobs provides in-process scheduling for periodic and trigger-aligned
// background jobs.
//
// It supports both instance-based scheduling through NewJobs and a process-wide
// scheduler exposed by package-level helpers.
//
// Main entry points:
//   - Job to register fixed-interval work
//   - CronJob to register jobs aligned to a daily trigger
//   - StartJobs to start registered global jobs
//   - RestartJobs to restart them without clearing definitions
//   - StopAllJobs to stop global jobs, optionally clearing them
//
// Jobs are not started when they are registered. They begin running only after
// StartJobs is called, and jobs added after startup begin executing
// immediately.
package jobs
