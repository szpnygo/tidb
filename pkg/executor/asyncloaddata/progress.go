// Copyright 2023 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package asyncloaddata

import (
	"encoding/json"
	"sync"

	"go.uber.org/atomic"
	"golang.org/x/exp/maps"
)

// LogicalImportProgress is the progress info of the logical import mode.
type LogicalImportProgress struct {
	// LoadedFileSize is the size of the data that's loaded in bytes. It's
	// larger than the actual loaded data size, but due to the fact that reading
	// is once-a-block and a block may generate multiple tasks that are
	// concurrently executed, we can't know the actual loaded data size easily.
	LoadedFileSize atomic.Int64
}

// PhysicalImportProgress is the progress info of the physical import mode.
type PhysicalImportProgress struct {
	colSizeMu  sync.Mutex
	colSizeMap map[int64]int64
}

// Progress is the progress of the LOAD DATA task.
type Progress struct {
	// SourceFileSize is the size of the source file in bytes. When we can't get
	// the size of the source file, it will be set to -1.
	// Currently, the value is read by seek(0, end), when LOAD DATA LOCAL we wrap
	// SimpleSeekerOnReadCloser on MySQL client connection which doesn't support
	// it.
	SourceFileSize          int64
	*LogicalImportProgress  `json:",inline"`
	*PhysicalImportProgress `json:",inline"`
	// LoadedRowCnt is the number of rows that has been loaded.
	// for physical mode, it's the number of rows that has been imported into TiKV.
	// in SHOW LOAD JOB we call it Imported_Rows, to make it compatible with 7.0,
	// the variable name is not changed.
	LoadedRowCnt atomic.Uint64
}

// NewProgress creates a new Progress.
// todo: better pass import mode, but it causes import cycle.
func NewProgress(logicalImport bool) *Progress {
	var li *LogicalImportProgress
	var pi *PhysicalImportProgress
	if logicalImport {
		li = &LogicalImportProgress{}
	} else {
		pi = &PhysicalImportProgress{
			colSizeMap: make(map[int64]int64),
		}
	}
	return &Progress{
		SourceFileSize:         -1,
		LogicalImportProgress:  li,
		PhysicalImportProgress: pi,
	}
}

// AddColSize adds the size of the column to the progress.
func (p *Progress) AddColSize(colSizeMap map[int64]int64) {
	p.colSizeMu.Lock()
	defer p.colSizeMu.Unlock()
	for key, value := range colSizeMap {
		p.colSizeMap[key] += value
	}
}

// GetColSize returns the size of the column.
func (p *Progress) GetColSize() map[int64]int64 {
	p.colSizeMu.Lock()
	defer p.colSizeMu.Unlock()
	return maps.Clone(p.colSizeMap)
}

// String implements the fmt.Stringer interface.
func (p *Progress) String() string {
	bs, _ := json.Marshal(p)
	return string(bs)
}

// ProgressFromJSON creates Progress from a JSON string.
func ProgressFromJSON(bs []byte) (*Progress, error) {
	var p Progress
	err := json.Unmarshal(bs, &p)
	return &p, err
}
