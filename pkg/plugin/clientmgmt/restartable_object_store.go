/*
Copyright 2021 the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clientmgmt

import (
	"context"
	"io"
	"time"

	"github.com/pkg/errors"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	objectstorev2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/objectstore/v2"
)

// restartableObjectStore is an object store for a given implementation (such as "aws"). It is associated with
// a restartableProcess, which may be shared and used to run multiple plugins. At the beginning of each method
// call, the restartableObjectStore asks its restartableProcess to restart itself if needed (e.g. if the
// process terminated for any reason), then it proceeds with the actual call.
type restartableObjectStore struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
	// config contains the data used to initialize the plugin. It is used to reinitialize the plugin in the event its
	// sharedPluginProcess gets restarted.
	config map[string]string
}

// newRestartableObjectStoreV2 returns a new restartableObjectStore for version 2.
func newRestartableObjectStoreV2(name string, sharedPluginProcess RestartableProcess) *restartableObjectStore {
	key := kindAndName{kind: framework.PluginKindObjectStoreV2, name: name}
	r := &restartableObjectStore{
		key:                 key,
		sharedPluginProcess: sharedPluginProcess,
	}

	// Register our reinitializer so we can reinitialize after a restart with r.config.
	sharedPluginProcess.addReinitializer(key, r)

	return r
}

// reinitialize reinitializes a re-dispensed plugin using the initial data passed to Init().
func (r *restartableObjectStore) reinitialize(dispensed interface{}) error {
	objectStore, ok := dispensed.(objectstorev2.ObjectStore)
	if !ok {
		return errors.Errorf("%T is not a ObjectStore!", dispensed)
	}

	return r.init(objectStore, r.config)
}

// getObjectStore returns the object store for this restartableObjectStore. It does *not* restart the
// plugin process.
func (r *restartableObjectStore) getObjectStore() (objectstorev2.ObjectStore, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	objectStore, ok := plugin.(objectstorev2.ObjectStore)
	if !ok {
		return nil, errors.Errorf("%T is not a ObjectStore!", plugin)
	}

	return objectStore, nil
}

// getDelegate restarts the plugin process (if needed) and returns the object store for this restartableObjectStore.
func (r *restartableObjectStore) getDelegate() (objectstorev2.ObjectStore, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getObjectStore()
}

// Init initializes the object store instance using config. If this is the first invocation, r stores config for future
// reinitialization needs. Init does NOT restart the shared plugin process. Init may only be called once.
func (r *restartableObjectStore) Init(config map[string]string) error {
	if r.config != nil {
		return errors.Errorf("already initialized")
	}

	// Not using getDelegate() to avoid possible infinite recursion
	delegate, err := r.getObjectStore()
	if err != nil {
		return err
	}

	r.config = config

	return r.init(delegate, config)
}

// InitV2 initializes the object store instance using config. If this is the first invocation, r stores config for future
// reinitialization needs. Init does NOT restart the shared plugin process. Init may only be called once.
func (r *restartableObjectStore) InitV2(ctx context.Context, config map[string]string) error {
	return r.Init(config)
}

// init calls Init on objectStore with config. This is split out from Init() so that both Init() and reinitialize() may
// call it using a specific ObjectStore.
func (r *restartableObjectStore) init(objectStore objectstorev2.ObjectStore, config map[string]string) error {
	return objectStore.Init(config)
}

// PutObject restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) PutObject(bucket string, key string, body io.Reader) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.PutObject(bucket, key, body)
}

// ObjectExists restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) ObjectExists(bucket, key string) (bool, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return false, err
	}
	return delegate.ObjectExists(bucket, key)
}

// GetObject restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) GetObject(bucket string, key string) (io.ReadCloser, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.GetObject(bucket, key)
}

// ListCommonPrefixes restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) ListCommonPrefixes(bucket string, prefix string, delimiter string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListCommonPrefixes(bucket, prefix, delimiter)
}

// ListObjects restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) ListObjects(bucket string, prefix string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListObjects(bucket, prefix)
}

// DeleteObject restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) DeleteObject(bucket string, key string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.DeleteObject(bucket, key)
}

// CreateSignedURL restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) CreateSignedURL(bucket string, key string, ttl time.Duration) (string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateSignedURL(bucket, key, ttl)
}

// Version 2
// PutObjectV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) PutObjectV2(ctx context.Context, bucket string, key string, body io.Reader) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.PutObjectV2(ctx, bucket, key, body)
}

// ObjectExistsV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) ObjectExistsV2(ctx context.Context, bucket, key string) (bool, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return false, err
	}
	return delegate.ObjectExistsV2(ctx, bucket, key)
}

// GetObjectV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) GetObjectV2(ctx context.Context, bucket string, key string) (io.ReadCloser, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.GetObjectV2(ctx, bucket, key)
}

// ListCommonPrefixesV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) ListCommonPrefixesV2(
	ctx context.Context, bucket string, prefix string, delimiter string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListCommonPrefixesV2(ctx, bucket, prefix, delimiter)
}

// ListObjectsV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) ListObjectsV2(ctx context.Context, bucket string, prefix string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListObjectsV2(ctx, bucket, prefix)
}

// DeleteObjectV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) DeleteObjectV2(ctx context.Context, bucket string, key string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.DeleteObjectV2(ctx, bucket, key)
}

// CreateSignedURLV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) CreateSignedURLV2(ctx context.Context, bucket string, key string, ttl time.Duration) (string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateSignedURLV2(ctx, bucket, key, ttl)
}
