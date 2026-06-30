package config

type Rest struct {
	Version       string              `yaml:"version"`
	Module        string              `yaml:"module"`
	ProjectPath   string              `yaml:"project_path"`
	Language      string              `yaml:"language"`
	GoVersion     string              `yaml:"go_version"`
	Environment   string              `yaml:"environment"`
	HTTP          HTTP                `yaml:"http"`
	SQL           Enabled             `yaml:"sql"`
	AutoSQLC      Enabled             `yaml:"auto_sqlc"`
	Mongo         Enabled             `yaml:"mongo"`
	Auth          Enabled             `yaml:"auth"`
	SafeReload    Enabled             `yaml:"safe_reload"`
	Logging       Logging             `yaml:"logging"`
	OpenAPI       OpenAPI             `yaml:"openapi"`
	Docker        Docker              `yaml:"docker"`
	Testing       Testing             `yaml:"testing"`
	Observability Observability       `yaml:"observability"`
	Features      ApplicationFeatures `yaml:"features"`
}

type HTTP struct {
	Framework        string          `yaml:"framework"`
	Host             string          `yaml:"host"`
	Port             int             `yaml:"port"`
	BasePath         string          `yaml:"base_path"`
	Timeouts         HTTPTimeouts    `yaml:"timeouts"`
	Limits           HTTPLimits      `yaml:"limits"`
	GracefulShutdown GeneratedSwitch `yaml:"graceful_shutdown"`
	Health           Health          `yaml:"health"`
	Middleware       Middleware      `yaml:"middleware"`
}

type HTTPTimeouts struct {
	ReadHeader string `yaml:"read_header"`
	Read       string `yaml:"read"`
	Write      string `yaml:"write"`
	Idle       string `yaml:"idle"`
	Shutdown   string `yaml:"shutdown"`
}

type HTTPLimits struct {
	MaxBodyBytes int64 `yaml:"max_body_bytes"`
}
type GeneratedSwitch struct {
	Enabled Enabled `yaml:"enabled"`
}
type Health struct {
	Enabled Enabled `yaml:"enabled"`
	Path    string  `yaml:"path"`
}
type Middleware struct {
	RequestID RequestID `yaml:"request_id"`
	CORS      CORS      `yaml:"cors"`
	Recovery  Recovery  `yaml:"recovery"`
}
type RequestID struct {
	Enabled Enabled `yaml:"enabled"`
	Header  string  `yaml:"header"`
}
type Recovery struct {
	Enabled       Enabled `yaml:"enabled"`
	ExposeDetails bool    `yaml:"expose_details"`
}
type CORS struct {
	Enabled          Enabled  `yaml:"enabled"`
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	ExposeHeaders    []string `yaml:"expose_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           string   `yaml:"max_age"`
}

type Logging struct {
	Enabled  Enabled           `yaml:"enabled"`
	Library  string            `yaml:"library"`
	Level    string            `yaml:"level"`
	Format   string            `yaml:"format"`
	Output   LoggingOutput     `yaml:"output"`
	Rotation LogRotation       `yaml:"rotation"`
	Fields   map[string]string `yaml:"fields"`
	Redact   []string          `yaml:"redact"`
}
type LoggingOutput struct {
	Type string `yaml:"type"`
	File string `yaml:"file"`
}
type LogRotation struct {
	Enabled    Enabled `yaml:"enabled"`
	MaxSizeMB  int     `yaml:"max_size_mb"`
	MaxBackups int     `yaml:"max_backups"`
	MaxAgeDays int     `yaml:"max_age_days"`
	Compress   bool    `yaml:"compress"`
}

type OpenAPI struct {
	Enabled         Enabled `yaml:"enabled"`
	Output          string  `yaml:"output"`
	Title           string  `yaml:"title"`
	Version         string  `yaml:"version"`
	Description     string  `yaml:"description"`
	ServerURL       string  `yaml:"server_url"`
	WithUI          Enabled `yaml:"with_ui"`
	UIPath          string  `yaml:"ui_path"`
	SpecPath        string  `yaml:"spec_path"`
	SecuritySchemes string  `yaml:"security_schemes"`
}

type Docker struct {
	Enabled            Enabled           `yaml:"enabled"`
	Output             string            `yaml:"output"`
	DockerignoreOutput string            `yaml:"dockerignore_output"`
	Compose            DockerCompose     `yaml:"compose"`
	BuildImage         string            `yaml:"build_image"`
	RuntimeImage       string            `yaml:"runtime_image"`
	Binary             string            `yaml:"binary"`
	Port               int               `yaml:"port"`
	User               string            `yaml:"user"`
	CGOEnabled         bool              `yaml:"cgo_enabled"`
	Healthcheck        DockerHealthcheck `yaml:"healthcheck"`
}
type DockerCompose struct {
	Enabled Enabled `yaml:"enabled"`
	Output  string  `yaml:"output"`
}
type DockerHealthcheck struct {
	Enabled  Enabled `yaml:"enabled"`
	Path     string  `yaml:"path"`
	Interval string  `yaml:"interval"`
	Timeout  string  `yaml:"timeout"`
	Retries  int     `yaml:"retries"`
}

type Testing struct {
	HandlerTests Enabled `yaml:"handler_tests"`
	Curl         Enabled `yaml:"curl"`
}
type Observability struct {
	Metrics Metrics `yaml:"metrics"`
}
type Metrics struct {
	Enabled   Enabled          `yaml:"enabled"`
	Provider  string           `yaml:"provider"`
	Path      string           `yaml:"path"`
	Namespace string           `yaml:"namespace"`
	Collect   MetricCollection `yaml:"collect"`
	Labels    []string         `yaml:"labels"`
}
type MetricCollection struct {
	HTTPRequests     bool `yaml:"http_requests"`
	RequestDuration  bool `yaml:"request_duration"`
	ResponseSize     bool `yaml:"response_size"`
	InFlightRequests bool `yaml:"in_flight_requests"`
}

type ApplicationFeatures struct {
	Makefile  GeneratedFile    `yaml:"makefile"`
	Gitignore GitignoreFeature `yaml:"gitignore"`
	Env       EnvFeature       `yaml:"env"`
	InitDB    GeneratedFile    `yaml:"init_db"`
	CI        GeneratedFile    `yaml:"ci"`
	CD        GeneratedFile    `yaml:"cd"`
}
type GeneratedFile struct {
	Enabled Enabled `yaml:"enabled"`
	Output  string  `yaml:"output"`
}
type GitignoreFeature struct {
	Enabled Enabled `yaml:"enabled"`
	Output  string  `yaml:"output"`
	Append  bool    `yaml:"append"`
}
type EnvFeature struct {
	Enabled          Enabled `yaml:"enabled"`
	Output           string  `yaml:"output"`
	GenerateLocalEnv bool    `yaml:"generate_local_env"`
}

type SQL struct {
	Version         string       `yaml:"version"`
	Database        string       `yaml:"database"`
	Connection      DBConnection `yaml:"db_connection"`
	ORM             Enabled      `yaml:"orm"`
	Engine          string       `yaml:"engine"`
	SQLC            SQLC         `yaml:"sqlc"`
	InitMigration   Enabled      `yaml:"init_migration"`
	MigrationEngine string       `yaml:"migration_engine"`
	MigrationOutput string       `yaml:"migration_output"`
}
type SQLC struct {
	Enabled Enabled `yaml:"enable"`
	Example Enabled `yaml:"rest_sqlc_example"`
	Path    string  `yaml:"sqlc_path"`
}
type DBConnection struct {
	DBName             string  `yaml:"db_name"`
	UserName           string  `yaml:"user_name"`
	UserPassword       string  `yaml:"user_password"`
	LegacyUserPassword string  `yaml:"usere_password"`
	PoolConnection     Enabled `yaml:"pool_connection"`
	Options            string  `yaml:"options"`
}
type Bundle struct {
	Dir   string
	Rest  Rest
	SQL   *SQL
	Mongo *Mongo
	Auth  *Auth
}

type Mongo struct {
	Version    string          `yaml:"version"`
	Connection MongoConnection `yaml:"connection"`
	Engine     string          `yaml:"engine"`
	Mongo      MongoSettings   `yaml:"mongo"`
	Generation MongoGeneration `yaml:"generation"`
}

type MongoConnection struct {
	URIEnv   string `yaml:"uri_env"`
	Database string `yaml:"database"`
	Timeout  string `yaml:"timeout"`
}

type MongoSettings struct {
	ModelsPath string `yaml:"models_path"`
}

type MongoGeneration struct {
	Package              string `yaml:"package"`
	Output               string `yaml:"output"`
	RepositoryOutput     string `yaml:"repository_output"`
	CreateIndexesOnStart bool   `yaml:"create_indexes_on_start"`
}

type Auth struct {
	Version        string             `yaml:"version"`
	Identity       AuthIdentity       `yaml:"identity"`
	Authentication AuthAuthentication `yaml:"authentication"`
	Authorization  AuthAuthorization  `yaml:"authorization"`
	Endpoints      []AuthEndpoint     `yaml:"endpoints"`
}

type AuthIdentity struct {
	Model         string `yaml:"model"`
	Table         string `yaml:"table"`
	IDField       string `yaml:"id_field"`
	UsernameField string `yaml:"username_field"`
	PasswordField string `yaml:"password_field"`
	RolesField    string `yaml:"roles_field"`
	ClaimsModel   string `yaml:"claims_model"`
}

type AuthAuthentication struct {
	Strategy string    `yaml:"strategy"`
	JWT      AuthJWT   `yaml:"jwt"`
	Basic    AuthBasic `yaml:"basic"`
}

type AuthJWT struct {
	Algorithm              string `yaml:"algorithm"`
	SigningKeyEnv          string `yaml:"signing_key_env"`
	VerificationKeyFileEnv string `yaml:"verification_key_file_env"`
	Issuer                 string `yaml:"issuer"`
	Audience               string `yaml:"audience"`
	AccessTokenTTL         string `yaml:"access_token_ttl"`
	RefreshToken           bool   `yaml:"refresh_token"`
	RefreshTokenStorage    string `yaml:"refresh_token_storage"`
	Leeway                 string `yaml:"leeway"`
	HeaderName             string `yaml:"header_name"`
	TokenPrefix            string `yaml:"token_prefix"`
}

type AuthBasic struct {
	UsernameEnv string   `yaml:"username_env"`
	PasswordEnv string   `yaml:"password_env"`
	Realm       string   `yaml:"realm"`
	Roles       []string `yaml:"roles"`
}

type AuthAuthorization struct {
	DefaultPolicy string `yaml:"default_policy"`
	RoleClaim     string `yaml:"role_claim"`
}

type AuthEndpoint struct {
	Name        string   `yaml:"name"`
	Method      string   `yaml:"method"`
	Path        string   `yaml:"path"`
	Public      bool     `yaml:"public"`
	RequireAuth bool     `yaml:"require_auth"`
	Roles       []string `yaml:"roles"`
}
