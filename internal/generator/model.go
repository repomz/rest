package generator

import "io"

type Options struct {
	SQLCPath string
	OutDir   string
	Features FeatureOptions
	Stdin    io.Reader
	Stdout   io.Writer
}

type FeatureOptions struct {
	HTTP    HTTPFeatures
	Logging LoggingFeatures
	OpenAPI OpenAPIFeatures
	Build   BuildFeatures
	Metrics MetricsFeatures
	Docker  DockerFeatures
}

type BuildFeatures struct {
	Configured       bool
	Makefile         bool
	HandlerTests     bool
	Curl             bool
	HTTPPort         int
	MakefilePath     string
	Gitignore        bool
	GitignorePath    string
	GitignoreAppend  bool
	Env              bool
	EnvPath          string
	GenerateLocalEnv bool
	ConfigPath       string
	InitDB           bool
	InitDBPath       string
	SafeReload       bool
	CI               bool
	CIPath           string
	CD               bool
	CDPath           string
	InitMigration    bool
	MigrationEngine  string
	MigrationsPath   string
	DBName           string
	DBUser           string
	DBPassword       string
	DBOptions        string
}

type HTTPFeatures struct {
	CORS                  bool
	AllowOrigins          []string
	AllowMethods          []string
	AllowHeaders          []string
	ExposeHeaders         []string
	AllowCredentials      bool
	CORSMaxAge            string
	Recovery              bool
	RecoveryExposeDetails bool
	RequestID             bool
	RequestIDHeader       string
	Host                  string
	Port                  int
	BasePath              string
	ReadHeaderTimeout     string
	ReadTimeout           string
	WriteTimeout          string
	IdleTimeout           string
	ShutdownTimeout       string
	MaxBodyBytes          int64
	GracefulShutdown      bool
	Health                bool
	HealthPath            string
}

type LoggingFeatures struct {
	Enabled    bool
	Library    string
	Level      string
	Format     string
	OutputType string
	OutputFile string
	Rotation   bool
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	Fields     map[string]string
	Redact     []string
}

type OpenAPIFeatures struct {
	Enabled         bool
	Output          string
	WithUI          bool
	Title           string
	Version         string
	Description     string
	ServerURL       string
	UIPath          string
	SpecPath        string
	SecuritySchemes string
}

type MetricsFeatures struct {
	Enabled          bool
	Provider         string
	Path             string
	Namespace        string
	HTTPRequests     bool
	RequestDuration  bool
	ResponseSize     bool
	InFlightRequests bool
	Labels           []string
}

type DockerFeatures struct {
	Enabled            bool
	Output             string
	DockerignoreOutput string
	BuildImage         string
	RuntimeImage       string
	Binary             string
	Port               int
	User               string
	CGOEnabled         bool
	Healthcheck        bool
	HealthPath         string
	HealthInterval     string
	HealthTimeout      string
	HealthRetries      int
}

type sqlcConfig struct {
	QueriesDirs []string
	SchemaDirs  []string
	DBPackage   string
	DBOut       string
}

type table struct {
	Name       string
	Singular   string
	Plural     string
	GoName     string
	GoPlural   string
	RouteBase  string
	Columns    []column
	CreateCols []column
	Endpoints  []endpoint
	Queries    querySet
}

type column struct {
	Name       string
	GoName     string
	JSONName   string
	GoType     string
	DBValue    string
	Nullable   bool
	Required   bool
	ReadOnly   bool
	NeedsSQL   bool
	NeedsTime  bool
	NeedsUUID  bool
	ValidCheck string
}

type querySet struct {
	Create    bool
	GetAll    bool
	GetByID   bool
	Delete    bool
	DeleteAll bool
}

type endpoint struct {
	TableName      string
	Name           string
	Method         string
	Path           string
	Query          string
	Result         string
	Params         []endpointParam
	BodyParams     []endpointParam
	NonBodyParams  []endpointParam
	NeedsTime      bool
	NeedsStrconv   bool
	NeedsUUID      bool
	QueryArgType   string
	QueryArgKind   string
	ReturnType     string
	ResponseType   string
	ZeroValue      string
	SampleReturn   string
	DomainResponse bool
	IsExec         bool
}

type endpointParam struct {
	Name       string
	GoName     string
	JSONName   string
	Source     string
	Type       string
	GoType     string
	Required   bool
	NeedsTime  bool
	NeedsUUID  bool
	NeedsInt   bool
	ValidCheck string
	DBExpr     string
}

type renderData struct {
	Module    string
	DBPackage string
	DBImport  string
	Tables    []table
	Table     table
	Queries   querySet
	Features  FeatureOptions
	OpenAPI   string
}
