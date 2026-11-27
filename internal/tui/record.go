package tui

import (
 "io"
 "os"

 "github.com/Routilet/c-observer/internal/observer"
 "github.com/Routilet/c-observer/internal/record"
)

const RecordVersion = record.RecordVersion
type ObservationRecord = record.ObservationRecord

func captureRecord(version string,cfg observer.LaunchConfig,run observer.RunResult) ObservationRecord {
 return record.Capture(version,cfg,run)
}
func DecodeRecord(r io.Reader)(ObservationRecord,error){return record.DecodeRecord(r)}
func LoadRecord(path string)(ObservationRecord,error){return record.LoadRecord(path)}
func SaveRecord(dir string,r ObservationRecord)(string,error){return record.SaveRecord(dir,r)}

// WithVersion supplies the actual CLI build version for live exports.
func (m Model) WithVersion(version string) Model {
	m.observerVersion = version
	return m
}

// NewRecorded reuses the normal replay UI with no engine, setup or tool lookup.
func NewRecorded(record ObservationRecord) (Model, error) {
	if err := record.Validate(); err != nil {
		return Model{}, err
	}
	return Model{
		replay: NewReplay(record.Run), offline: true, record: &record,
		config: observer.LaunchConfig{Stdin: record.Stdin, ExpectedStdout: record.ExpectedStdout, Comparison: record.Comparison},
		width:  100, height: 30, focus: 1, consoleTab: 1,
	}, nil
}

func (m *Model) exportRecord() {
	if m.record == nil || m.offline || m.demo {
		m.notice = "没有可导出的现场观察记录。"
		return
	}
	dir, err := os.UserHomeDir()
	if err == nil {
		m.exportPath, err = SaveRecord(dir, *m.record)
	}
	if err != nil {
		m.notice = "导出失败：" + SafeText(err.Error(), false)
	} else {
		m.notice = ""
	}
	m.mode = "record-result"
}
