package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds application configuration (env + Viper).
type Config struct {
	Env                string
	Port               string
	SessionSecret      string
	DatabaseURL        string
	RedisURL           string
	SupabaseURL        string // e.g. https://xwsiuytkbefejvoqpjyg.supabase.co — used for storage sign URLs and public URLs
	SupabaseSecretKey  string // must be service_role key (Dashboard → API), not anon key
	StripeSecretKey    string
	StripeWebhookSecret string
	FrontendURLEndsWith string
	DevPassword        string
	AllowCrossSiteDev  bool
	HealthAdminKey       string
	ICRAPIKey            string
	SendinblueAPIKey     string // SENDINBLUE_API_KEY for welcome/notification emails (Brevo)
	MailFrom             string // MAIL_FROM sender email (default noreply@troo.earth)
	InviteBaseURL        string // Base URL for invite links (e.g. https://atlas.troo.earth), same logic as Express
}

// Load loads config from env and optional .env file.
func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	_ = viper.ReadInConfig()

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	port := viper.GetString("PORT")
	if port == "" {
		port = "8080"
	}
	env := viper.GetString("NODE_ENV")
	if env == "" {
		env = viper.GetString("APP_ENV")
	}
	if env == "" {
		env = "development"
	}

	dbURL := viper.GetString("DATABASE_URL_DEV")
	if env == "production" {
		dbURL = viper.GetString("DATABASE_URL_PROD")
	} else if env == "test" {
		dbURL = viper.GetString("DATABASE_URL_TEST")
	}
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL_DEV")
	}

	return &Config{
		Env:                 env,
		Port:                port,
		SessionSecret:       viper.GetString("SESSION_SECRET"),
		DatabaseURL:         dbURL,
		RedisURL:            viper.GetString("REDIS_URL"),
		SupabaseURL:         viper.GetString("SUPABASE_URL"),
		SupabaseSecretKey:   viper.GetString("SUPABASE_SECRET_KEY"),
		StripeSecretKey:     viper.GetString("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: viper.GetString("STRIPE_WEBHOOK_SECRET"),
		FrontendURLEndsWith: viper.GetString("FRONTEND_URL_ENDS_WITH"),
		DevPassword:         viper.GetString("DEV_PASSWORD"),
		AllowCrossSiteDev:   strings.EqualFold(viper.GetString("ALLOW_CROSS_SITE_DEV"), "true"),
		HealthAdminKey:       viper.GetString("HEALTH_ADMIN_KEY"),
		ICRAPIKey:            viper.GetString("ICR_API_KEY"),
		SendinblueAPIKey:     viper.GetString("SENDINBLUE_API_KEY"),
		MailFrom:             viper.GetString("MAIL_FROM"),
		InviteBaseURL:        inviteBaseURL(viper.GetString("INVITE_BASE_URL")),
	}, nil
}

func inviteBaseURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "https://atlas.troo.earth"
	}
	return s
}

