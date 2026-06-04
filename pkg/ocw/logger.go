package ocw

// Logger is the contract used by the engine, compiler, and docker runtime.
type Logger interface {
	Debug(msg string, fields map[string]any)
	Info(msg string, fields map[string]any)
	Warn(msg string, fields map[string]any)
	Error(msg string, fields map[string]any)
	Event(ev Event)
}

// LevelFilter wraps a Logger and suppresses Debug when allowDebug is false.
type LevelFilter struct {
	allowDebug bool
	logger     Logger
}

func NewLevelFilter(allowDebug bool, logger Logger) *LevelFilter {
	return &LevelFilter{
		allowDebug: allowDebug,
		logger:     logger,
	}
}

func (l *LevelFilter) Debug(msg string, fields map[string]any) {
	if l.allowDebug {
		l.logger.Debug(msg, fields)
	}
}

func (l *LevelFilter) Info(msg string, fields map[string]any)  { l.logger.Info(msg, fields) }
func (l *LevelFilter) Warn(msg string, fields map[string]any)  { l.logger.Warn(msg, fields) }
func (l *LevelFilter) Error(msg string, fields map[string]any) { l.logger.Error(msg, fields) }
func (l *LevelFilter) Event(ev Event)                           { l.logger.Event(ev) }
