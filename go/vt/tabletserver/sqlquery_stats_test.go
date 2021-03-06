// Copyright 2015, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tabletserver

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/youtube/vitess/go/sqltypes"
	"github.com/youtube/vitess/go/vt/callinfo"
	"golang.org/x/net/context"
)

func TestSqlQueryStats(t *testing.T) {
	logStats := newSqlQueryStats("test", context.Background())
	logStats.AddRewrittenSql("sql1", time.Now())

	if !strings.Contains(logStats.RewrittenSql(), "sql1") {
		t.Fatalf("RewrittenSql should contains sql: sql1")
	}

	if logStats.SizeOfResponse() != 0 {
		t.Fatalf("there is no rows in log stats, estimated size should be 0 bytes")
	}

	logStats.Rows = [][]sqltypes.Value{[]sqltypes.Value{sqltypes.MakeString([]byte("a"))}}
	if logStats.SizeOfResponse() <= 0 {
		t.Fatalf("log stats has some rows, should have positive response size")
	}

	params := map[string][]string{"full": []string{}}

	logStats.Format(url.Values(params))
}

func TestSqlQueryStatsFormatBindVariables(t *testing.T) {
	logStats := newSqlQueryStats("test", context.Background())
	logStats.BindVariables = make(map[string]interface{})
	logStats.BindVariables["key_1"] = "val_1"
	logStats.BindVariables["key_2"] = 789

	formattedStr := logStats.FmtBindVariables(true)
	if !strings.Contains(formattedStr, "key_1") ||
		!strings.Contains(formattedStr, "val_1") {
		t.Fatalf("bind variable 'key_1': 'val_1' is not formatted")
	}
	if !strings.Contains(formattedStr, "key_2") ||
		!strings.Contains(formattedStr, "789") {
		t.Fatalf("bind variable 'key_2': '789' is not formatted")
	}

	logStats.BindVariables["key_3"] = []byte("val_3")
	formattedStr = logStats.FmtBindVariables(false)
	if !strings.Contains(formattedStr, "key_1") {
		t.Fatalf("bind variable 'key_1' is not formatted")
	}
	if !strings.Contains(formattedStr, "key_2") ||
		!strings.Contains(formattedStr, "789") {
		t.Fatalf("bind variable 'key_2': '789' is not formatted")
	}
	if !strings.Contains(formattedStr, "key_3") {
		t.Fatalf("bind variable 'key_3' is not formatted")
	}
}

func TestSqlQueryStatsFormatQuerySources(t *testing.T) {
	logStats := newSqlQueryStats("test", context.Background())
	if logStats.FmtQuerySources() != "none" {
		t.Fatalf("should return none since log stats does not have any query source, but got: %s", logStats.FmtQuerySources())
	}

	logStats.QuerySources |= QuerySourceMySQL
	if !strings.Contains(logStats.FmtQuerySources(), "mysql") {
		t.Fatalf("'mysql' should be in formated query sources")
	}

	logStats.QuerySources |= QuerySourceRowcache
	if !strings.Contains(logStats.FmtQuerySources(), "rowcache") {
		t.Fatalf("'rowcache' should be in formated query sources")
	}

	logStats.QuerySources |= QuerySourceConsolidator
	if !strings.Contains(logStats.FmtQuerySources(), "consolidator") {
		t.Fatalf("'consolidator' should be in formated query sources")
	}
}

func TestSqlQueryStatsContextHTML(t *testing.T) {
	html := "HtmlContext"
	callInfo := &fakeCallInfo{
		html: html,
	}
	ctx := callinfo.NewContext(context.Background(), callInfo)
	logStats := newSqlQueryStats("test", ctx)
	if string(logStats.ContextHTML()) != html {
		t.Fatalf("expect to get html: %s, but got: %s", html, string(logStats.ContextHTML()))
	}
}

func TestSqlQueryStatsErrorStr(t *testing.T) {
	logStats := newSqlQueryStats("test", context.Background())
	if logStats.ErrorStr() != "" {
		t.Fatalf("should not get error in stats, but got: %s", logStats.ErrorStr())
	}
	errStr := "unknown error"
	logStats.Error = fmt.Errorf(errStr)
	if logStats.ErrorStr() != errStr {
		t.Fatalf("expect to get error string: %s, but got: %s", errStr, logStats.ErrorStr())
	}
}

func TestSqlQueryStatsRemoteAddrUsername(t *testing.T) {
	logStats := newSqlQueryStats("test", context.Background())
	addr, user := logStats.RemoteAddrUsername()
	if addr != "" {
		t.Fatalf("remote addr should be empty")
	}
	if user != "" {
		t.Fatalf("username should be empty")
	}

	remoteAddr := "1.2.3.4"
	username := "vt"
	callInfo := &fakeCallInfo{
		remoteAddr: remoteAddr,
		username:   username,
	}
	ctx := callinfo.NewContext(context.Background(), callInfo)
	logStats = newSqlQueryStats("test", ctx)
	addr, user = logStats.RemoteAddrUsername()
	if addr != remoteAddr {
		t.Fatalf("expected to get remote addr: %s, but got: %s", remoteAddr, addr)
	}
	if user != username {
		t.Fatalf("expected to get username: %s, but got: %s", username, user)
	}
}
