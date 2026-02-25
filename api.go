package consolex

import (
	"os"

	"github.com/VexoraDevelopment/consolex/cmdline"
	"github.com/VexoraDevelopment/consolex/logging"
	"github.com/VexoraDevelopment/consolex/style"
)

type Chalk = style.Chalk
type Theme = style.Theme

func New() Chalk      { return style.New() }
func Disabled() Chalk { return style.Disabled() }

func DefaultTheme() Theme { return style.DefaultTheme() }
func NordTheme() Theme    { return style.NordTheme() }
func SunsetTheme() Theme  { return style.SunsetTheme() }

type LoggerConfig = logging.LoggerConfig
type Profile = logging.Profile
type LogRecord = logging.LogRecord
type RecordField = logging.RecordField
type Processor = logging.Processor
type ProcessorFunc = logging.ProcessorFunc
type Renderer = logging.Renderer
type RendererFunc = logging.RendererFunc
type FieldStyleProvider = logging.FieldStyleProvider
type FieldStyleFunc = logging.FieldStyleFunc
type StaticFieldProvider = logging.StaticFieldProvider
type FieldTransformer = logging.FieldTransformer
type FieldTransformFunc = logging.FieldTransformFunc

func DefaultProfile() Profile                 { return logging.DefaultProfile() }
func ParseTextLogLine(line string) *LogRecord { return logging.ParseTextLogLine(line) }

func SetupDefaultSlog(cfg LoggerConfig) (*os.File, error) {
	return logging.SetupDefaultSlog(cfg)
}

func RotateAndCompressLog(srcPath, archiveDir string) error {
	return logging.RotateAndCompressLog(srcPath, archiveDir)
}

func ColorizeLogLine(line string) string { return logging.ColorizeLogLine(line) }
func StripANSI(s string) string          { return style.StripANSI(s) }

type Command = cmdline.Command
type Options = cmdline.Options
type Loop = cmdline.Loop

func NewLoop(opts Options) *Loop { return cmdline.NewLoop(opts) }
