package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestInformationCommands(t *testing.T) {
	var b bytes.Buffer
	if err:=run([]string{"--version"},&b);err!=nil || b.String()!=version+"\n" {t.Fatal(err,b.String())}
	b.Reset()
	if err:=run([]string{"--capabilities"},&b);err!=nil {t.Fatal(err)}
	var info struct{SessionSchemaVersions []int `json:"sessionSchemaVersions"`}
	if err:=json.Unmarshal(b.Bytes(),&info);err!=nil || len(info.SessionSchemaVersions)!=1 || info.SessionSchemaVersions[0]!=1 {t.Fatal(b.String())}
}
