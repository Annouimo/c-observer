package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"os"
	"path/filepath"
)

func TestInformationCommands(t *testing.T) {
	var b bytes.Buffer
	if err:=run([]string{"--version"},&b);err!=nil || b.String()!=version+"\n" {t.Fatal(err,b.String())}
	b.Reset()
	if err:=run([]string{"--capabilities"},&b);err!=nil {t.Fatal(err)}
	var info struct{SessionSchemaVersions []int `json:"sessionSchemaVersions"`}
	if err:=json.Unmarshal(b.Bytes(),&info);err!=nil || len(info.SessionSchemaVersions)!=1 || info.SessionSchemaVersions[0]!=1 {t.Fatal(b.String())}
}

func TestReplayCLIValidation(t *testing.T) {
	var out bytes.Buffer
	if err:=run([]string{"--replay","missing.json","--compiler","gcc"},&out);err==nil {t.Fatal("replay accepts tool override")}
	if err:=run([]string{"--replay","missing.json","--session","session.json"},&out);err==nil {t.Fatal("replay accepts session")}
	path:=filepath.Join(t.TempDir(),"bad.json")
	if err:=os.WriteFile(path,[]byte("{}"),0600);err!=nil {t.Fatal(err)}
	if _,err:=loadReplay(path);err==nil {t.Fatal("invalid record accepted")}
}
