package repository

import (
	"context"

	"bennsimon.me/configmirror-operator/pkg/utils"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

type ConfigMapRepository interface {
	AddOrUpdate(ctx context.Context, cm *v1.ConfigMap) error
}

type ConfigMapRepositoryImpl struct {
	Database *pgxpool.Pool
}

func NewConfigMapRepositoryImpl(db *pgxpool.Pool) *ConfigMapRepositoryImpl {
	return &ConfigMapRepositoryImpl{Database: db}
}

func (cmr *ConfigMapRepositoryImpl) AddOrUpdate(ctx context.Context, cm *v1.ConfigMap) error {
	data, err := json.Marshal(cm)

	if err != nil {
		return err
	}
	_, err = cmr.Database.Exec(ctx, `
		INSERT INTO configmirror.configmaps (name, source_namespace, destination_namespace, configmirror, json_data)
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT(name, destination_namespace) DO UPDATE SET json_data=EXCLUDED.json_data, configmirror=EXCLUDED.configmirror, updated_at=now();
		
	`, cm.GetName(), cm.GetAnnotations()[utils.GetAnnotationKey(utils.SourceNamespace)], cm.GetNamespace(), cm.GetAnnotations()[utils.GetAnnotationKey(utils.ManagedBy)], string(data))
	if err != nil {
		return err
	}

	return nil
}
