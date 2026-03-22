// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package viperdata

type TraceAtribute string

const (
	AppAtribute                                TraceAtribute = "app.name"
	AppVersionAtribute                         TraceAtribute = "app.version"
	GinLoggerWithConfigEnabledAtribute         TraceAtribute = "server.gin.LoggerWithConfig.enabled"
	GinLoggerWithConfigSkipPathsAtribute       TraceAtribute = "server.gin.LoggerWithConfig.SkipPaths"
	GinLoggerWithConfigSkipQueryStringAtribute TraceAtribute = "server.gin.LoggerWithConfig.SkipQueryString"
	LoggerModeTestAtribute                     TraceAtribute = "logger.modeTest"
	LoggerLevelAtribute                        TraceAtribute = "logger.level"
	LoggerIgnoredHeadersAtribute               TraceAtribute = "logger.ignoredHeaders"
	LoggerFormatterAtribute                    TraceAtribute = "logger.formatter"
	LoggerFormatDateAtribute                   TraceAtribute = "logger.formatDate"
	LoggerRotateEnableAtribute                 TraceAtribute = "logger.rotate.enable"
	LoggerRotateMaxSizeAtribute                TraceAtribute = "logger.rotate.maxSize"
	LoggerRotateMaxBackupsAtribute             TraceAtribute = "logger.rotate.maxBackups"
	LoggerRotateMaxAgeAtribute                 TraceAtribute = "logger.rotate.maxAge"
	LoggerCompressMaxAgeAtribute               TraceAtribute = "logger.rotate.compress"
)
