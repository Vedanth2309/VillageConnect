package config

import "testing"

func TestValidateRequiresDeploymentJWTSecretInRelease(t *testing.T) {
	cfg := Config{
		Port: "9090", MongoURI: "mongodb://localhost:27017",
		MongoDatabase: "villageconnect", ElasticsearchIndex: "products",
		JWTSecret: "replace-this-with-a-random-secret-of-at-least-32-characters",
	}
	if err := cfg.Validate("release"); err == nil {
		t.Fatal("release configuration accepted the sample JWT secret")
	}

	cfg.JWTSecret = "a-unique-secret-for-this-deployment-with-more-than-32-chars"
	if err := cfg.Validate("release"); err != nil {
		t.Fatalf("release configuration rejected a valid secret: %v", err)
	}
}
