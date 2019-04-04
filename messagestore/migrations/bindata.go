// Code generated by go-bindata.
// sources:
// 000001_add_received_messages_table.down.db.sql
// 000001_add_received_messages_table.up.db.sql
// DO NOT EDIT!

package migrations

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var __000001_add_received_messages_tableDownDbSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x72\x09\xf2\x0f\x50\x08\x71\x74\xf2\x71\x55\x28\xcf\xc8\x2c\x2e\x48\x2d\x8a\x2f\x4a\x4d\x4e\xcd\x2c\x4b\x4d\x89\xcf\x4d\x2d\x2e\x4e\x4c\x4f\x2d\xb6\xe6\x02\x04\x00\x00\xff\xff\x8d\xe2\x8e\x2d\x26\x00\x00\x00")

func _000001_add_received_messages_tableDownDbSqlBytes() ([]byte, error) {
	return bindataRead(
		__000001_add_received_messages_tableDownDbSql,
		"000001_add_received_messages_table.down.db.sql",
	)
}

func _000001_add_received_messages_tableDownDbSql() (*asset, error) {
	bytes, err := _000001_add_received_messages_tableDownDbSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "000001_add_received_messages_table.down.db.sql", size: 38, mode: os.FileMode(436), modTime: time.Unix(1554364911, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var __000001_add_received_messages_tableUpDbSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x72\x0e\x72\x75\x0c\x71\x55\x08\x71\x74\xf2\x71\x55\x28\xcf\xc8\x2c\x2e\x48\x2d\x8a\x2f\x4a\x4d\x4e\xcd\x2c\x4b\x4d\x89\xcf\x4d\x2d\x2e\x4e\x4c\x4f\x2d\x56\xd0\xe0\xca\x48\x2c\xce\x50\x08\x73\x0c\x72\xf6\x70\x0c\xd2\x30\x36\xd2\x54\x08\x08\xf2\xf4\x75\x0c\x8a\x54\xf0\x76\x8d\x54\xf0\xf3\x0f\x51\xf0\x0b\xf5\xf1\xd1\xe1\x4a\xca\x4f\xa9\x54\x70\xf2\xf1\x77\xe2\xd2\xb4\xe6\x02\x04\x00\x00\xff\xff\xbd\x3c\x6d\xc2\x5d\x00\x00\x00")

func _000001_add_received_messages_tableUpDbSqlBytes() ([]byte, error) {
	return bindataRead(
		__000001_add_received_messages_tableUpDbSql,
		"000001_add_received_messages_table.up.db.sql",
	)
}

func _000001_add_received_messages_tableUpDbSql() (*asset, error) {
	bytes, err := _000001_add_received_messages_tableUpDbSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "000001_add_received_messages_table.up.db.sql", size: 93, mode: os.FileMode(436), modTime: time.Unix(1554364640, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"000001_add_received_messages_table.down.db.sql": _000001_add_received_messages_tableDownDbSql,
	"000001_add_received_messages_table.up.db.sql": _000001_add_received_messages_tableUpDbSql,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}
var _bintree = &bintree{nil, map[string]*bintree{
	"000001_add_received_messages_table.down.db.sql": &bintree{_000001_add_received_messages_tableDownDbSql, map[string]*bintree{}},
	"000001_add_received_messages_table.up.db.sql": &bintree{_000001_add_received_messages_tableUpDbSql, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}

