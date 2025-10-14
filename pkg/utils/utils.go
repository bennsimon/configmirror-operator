package utils

import (
	"fmt"
	"net/url"
	"os"
)

const (
	Api                       = "bennsimon.github.io"
	MirrorKey                 = "mirror"
	SourceNamespace           = "sourceNamespace"
	ControllerName            = "configmirror-controller"
	ManagedBy                 = "managed-by"
	CmDatabaseHostKey         = "CM_DATABASE_HOST"
	CmDatabasePasswordKey     = "CM_DATABASE_PASSWORD"
	CmDatabaseUsernameKey     = "CM_DATABASE_USERNAME"
	CmDatabasePortKey         = "CM_DATABASE_PORT"
	CmDatabaseDatabaseNameKey = "CM_DATABASE_NAME"
	SaveReplicationActionKey  = "SAVE_REPLICATION_ACTION"
)

func GetAnnotationKey(key string) string {
	return fmt.Sprintf("%s/%s", Api, key)
}

func GetFieldOwner() string {
	return fmt.Sprintf("%s/%s", Api, ControllerName)
}

func GetSystemEnv(key string) (string, error) {
	str, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("environment variable `%s` not set", key)
	}
	return str, nil
}

func SaveReplicationAction() bool {
	saveReplicationAction, ok := os.LookupEnv(SaveReplicationActionKey)
	return ok && saveReplicationAction == "true"
}

func GetDatabaseUrl() (string, error) {
	dbUsername, err := GetSystemEnv(CmDatabaseUsernameKey)
	if err != nil {
		return "", err
	}
	dbPassword, err := GetSystemEnv(CmDatabasePasswordKey)
	if err != nil {
		return "", err
	}
	dbHost, err := GetSystemEnv(CmDatabaseHostKey)
	if err != nil {
		return "", err
	}

	dbPort, err := GetSystemEnv(CmDatabasePortKey)
	if err != nil {
		return "", err
	}

	dbName, err := GetSystemEnv(CmDatabaseDatabaseNameKey)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", dbUsername, url.QueryEscape(dbPassword), dbHost, dbPort, dbName), nil
}
