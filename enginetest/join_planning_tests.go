// Copyright 2022 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package enginetest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/enginetest/scriptgen/setup"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/plan"
	"github.com/dolthub/go-mysql-server/sql/transform"
)

type JoinPlanTest struct {
	q     string
	types []plan.JoinType
	exp   []sql.Row
	order []string
	skip  bool
}

var JoinPlanningTests = []struct {
	name  string
	setup []string
	tests []JoinPlanTest
}{
	{
		name: "merge join unary index",
		setup: []string{
			"CREATE table xy (x int primary key, y int, index y_idx(y));",
			"create table rs (r int primary key, s int, index s_idx(s));",
			"CREATE table uv (u int primary key, v int);",
			"CREATE table ab (a int primary key, b int);",
			"insert into xy values (1,0), (2,1), (0,2), (3,3);",
			"insert into rs values (0,0), (1,0), (2,0), (4,4), (5,4);",
			"insert into uv values (0,1), (1,1), (2,2), (3,2);",
			"insert into ab values (0,2), (1,2), (2,2), (3,1);",
			"update information_schema.statistics set cardinality = 1000 where table_name in ('ab', 'rs', 'xy', 'uv');",
		},
		tests: []JoinPlanTest{
			{
				q:     "select u,a,y from uv join (select /*+ JOIN_ORDER(ab, xy) */ * from ab join xy on y = a) r on u = r.a order by 1",
				types: []plan.JoinType{plan.JoinTypeLookup, plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 0, 0}, {1, 1, 1}, {2, 2, 2}, {3, 3, 3}},
			},
			{
				q:     "select /*+ JOIN_ORDER(ab, xy) */ * from ab join xy on y = a order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 2, 1, 0}, {1, 2, 2, 1}, {2, 2, 0, 2}, {3, 1, 3, 3}},
			},
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs left join xy on y = s order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeLeftOuterMerge},
				exp:   []sql.Row{{0, 0, 1, 0}, {1, 0, 1, 0}, {2, 0, 1, 0}, {4, 4, nil, nil}, {5, 4, nil, nil}},
			},
			{
				// extra join condition does not filter left-only rows
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs left join xy on y = s and y+s = 0 order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeLeftOuterMerge},
				exp:   []sql.Row{{0, 0, 1, 0}, {1, 0, 1, 0}, {2, 0, 1, 0}, {4, 4, nil, nil}, {5, 4, nil, nil}},
			},
			{
				// extra join condition does not filter left-only rows
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs left join xy on y+2 = s and s-y = 2 order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeLeftOuterMerge},
				exp:   []sql.Row{{0, 0, nil, nil}, {1, 0, nil, nil}, {2, 0, nil, nil}, {4, 4, 0, 2}, {5, 4, 0, 2}},
			},
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y = r order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 0, 1, 0}, {1, 0, 2, 1}, {2, 0, 0, 2}},
			},
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on r = y order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 0, 1, 0}, {1, 0, 2, 1}, {2, 0, 0, 2}},
			},
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y = s order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 0, 1, 0}, {1, 0, 1, 0}, {2, 0, 1, 0}},
			},
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y = s and y = r order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 0, 1, 0}},
			},
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y+2 = s order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{{4, 4, 0, 2}, {5, 4, 0, 2}},
			},
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y = s-1 order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeLookup},
				exp:   []sql.Row{{4, 4, 3, 3}, {5, 4, 3, 3}},
			},
			//{
			// TODO: cannot hash join on compound expressions
			//	q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y = mod(s,2) order by 1, 3",
			//	types: []plan.JoinType{plan.JoinTypeInner},
			//	exp:   []sql.Row{{0,0,1,0},{0, 0, 1, 0},{2,0,1,0},{4,4,1,0}},
			//},
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on 2 = s+y order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeInner},
				exp:   []sql.Row{{0, 0, 0, 2}, {1, 0, 0, 2}, {2, 0, 0, 2}},
			},
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y > s+2 order by 1, 3",
				types: []plan.JoinType{plan.JoinTypeInner},
				exp:   []sql.Row{{0, 0, 3, 3}, {1, 0, 3, 3}, {2, 0, 3, 3}},
			},
		},
	},
	{
		name: "merge join multi match",
		setup: []string{
			"CREATE table xy (x int primary key, y int, index y_idx(y));",
			"create table rs (r int primary key, s int, index s_idx(s));",
			"insert into xy values (1,0), (2,1), (0,8), (3,7), (5,4), (4,0);",
			"insert into rs values (0,0),(2,3),(3,0), (4,8), (5,4);",
			"update information_schema.statistics set cardinality = 1000 where table_name in ('rs', 'xy');",
		},
		tests: []JoinPlanTest{
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y = s order by 1,3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 0, 1, 0}, {0, 0, 4, 0}, {3, 0, 1, 0}, {3, 0, 4, 0}, {4, 8, 0, 8}, {5, 4, 5, 4}},
			},
		},
	},
	{
		name: "merge join zero rows",
		setup: []string{
			"CREATE table xy (x int primary key, y int, index y_idx(y));",
			"create table rs (r int primary key, s int, index s_idx(s));",
			"insert into xy values (1,0);",
			"update information_schema.statistics set cardinality = 10 where table_name = 'xy';",
			"update information_schema.statistics set cardinality = 1000000000 where table_name = 'rs';",
		},
		tests: []JoinPlanTest{
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y = s order by 1,3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{},
			},
		},
	},
	{
		name: "merge join multi arity",
		setup: []string{
			"CREATE table xy (x int primary key, y int, index yx_idx(y,x));",
			"create table rs (r int primary key, s int, index s_idx(s));",
			"insert into xy values (1,0), (2,1), (0,8), (3,7), (5,4), (4,0);",
			"insert into rs values (0,0),(2,3),(3,0), (4,8), (5,4);",
			"update information_schema.statistics set cardinality = 1000 where table_name in ('xy', 'rs');",
		},
		tests: []JoinPlanTest{
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y = s order by 1,3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 0, 1, 0}, {0, 0, 4, 0}, {3, 0, 1, 0}, {3, 0, 4, 0}, {4, 8, 0, 8}, {5, 4, 5, 4}},
			},
		},
	},
	{
		name: "merge join keyless index",
		setup: []string{
			"CREATE table xy (x int, y int, index yx_idx(y,x));",
			"create table rs (r int, s int, index s_idx(s));",
			"insert into xy values (1,0), (2,1), (0,8), (3,7), (5,4), (4,0);",
			"insert into rs values (0,0),(2,3),(3,0), (4,8), (5,4);",
			"update information_schema.statistics set cardinality = 1000 where table_name in ('xy', 'rs');",
		},
		tests: []JoinPlanTest{
			{
				q:     "select /*+ JOIN_ORDER(rs, xy) */ * from rs join xy on y = s order by 1,3",
				types: []plan.JoinType{plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 0, 1, 0}, {0, 0, 4, 0}, {3, 0, 1, 0}, {3, 0, 4, 0}, {4, 8, 0, 8}, {5, 4, 5, 4}},
			},
		},
	},
	{
		name: "partial [lookup] join tests",
		setup: []string{
			"CREATE table xy (x int primary key, y int);",
			"create table rs (r int primary key, s int);",
			"CREATE table uv (u int primary key, v int);",
			"CREATE table ab (a int primary key, b int);",
			"insert into xy values (1,0), (2,1), (0,2), (3,3);",
			"insert into rs values (0,0), (1,0), (2,0), (4,4);",
			"insert into uv values (0,1), (1,1), (2,2), (3,2);",
			"insert into ab values (0,2), (1,2), (2,2), (3,1);",
			"update information_schema.statistics set cardinality = 100 where table_name in ('xy', 'rs', 'uv', 'ab');",
		},
		tests: []JoinPlanTest{
			{
				q:     "select * from xy where y+1 not in (select u from uv);",
				types: []plan.JoinType{plan.JoinTypeAntiLookup},
				exp:   []sql.Row{{3, 3}},
			},
			{
				q:     "select * from xy where x not in (select u from uv where u not in (select a from ab where a not in (select r from rs where r = 1))) order by 1;",
				types: []plan.JoinType{plan.JoinTypeAnti, plan.JoinTypeAnti, plan.JoinTypeAntiLookup},
				exp:   []sql.Row{{0, 2}, {2, 1}, {3, 3}},
			},
			{
				q:     "select * from xy where x != (select r from rs where r = 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeAnti},
				exp:   []sql.Row{{0, 2}, {2, 1}, {3, 3}},
			},
			{
				// anti join will be cross-join-right, be passed non-nil parent row
				q:     "select x,a from ab, (select * from xy where x != (select r from rs where r = 1) order by 1) sq where x = 2 and b = 2 order by 1,2;",
				types: []plan.JoinType{plan.JoinTypeCross, plan.JoinTypeAnti},
				exp:   []sql.Row{{2, 0}, {2, 1}, {2, 2}},
			},
			{
				// scope and parent row are non-nil
				q: `
select * from uv where u > (
  select x from ab, (
    select x from xy where x != (
      select r from rs where r = 1
    ) order by 1
  ) sq
  order by 1 limit 1
)
order by 1;`,
				types: []plan.JoinType{plan.JoinTypeSemi, plan.JoinTypeCross, plan.JoinTypeAnti},
				exp:   []sql.Row{{1, 1}, {2, 2}, {3, 2}},
			},
			{
				// cast prevents scope merging
				q:     "select * from xy where x != (select cast(r as signed) from rs where r = 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeAnti},
				exp:   []sql.Row{{0, 2}, {2, 1}, {3, 3}},
			},
			{
				// order by will be discarded
				q:     "select * from xy where x != (select r from rs where r = 1 order by 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeAnti},
				exp:   []sql.Row{{0, 2}, {2, 1}, {3, 3}},
			},
			{
				// limit prevents scope merging
				q:     "select * from xy where x != (select r from rs where r = 1 limit 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeAnti},
				exp:   []sql.Row{{0, 2}, {2, 1}, {3, 3}},
			},
			{
				q:     "select * from xy where y-1 in (select u from uv) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemiLookup},
				exp:   []sql.Row{{0, 2}, {2, 1}, {3, 3}},
			},
			{
				// semi join will be right-side, be passed non-nil parent row
				q:     "select x,a from ab, (select * from xy where x = (select r from rs where r = 1) order by 1) sq order by 1,2",
				types: []plan.JoinType{plan.JoinTypeCross, plan.JoinTypeRightSemiLookup},
				exp:   []sql.Row{{1, 0}, {1, 1}, {1, 2}, {1, 3}},
			},
			//{
			// scope and parent row are non-nil
			// TODO: subquery alias unable to track parent row from a different scope
			//				q: `
			//select * from uv where u > (
			//  select x from ab, (
			//    select x from xy where x = (
			//      select r from rs where r = 1
			//    ) order by 1
			//  ) sq
			//  order by 1 limit 1
			//)
			//order by 1;`,
			//types: []plan.JoinType{plan.JoinTypeCross, plan.JoinTypeRightSemiLookup},
			//exp:   []sql.Row{{2, 2}, {3, 2}},
			//},
			{
				q:     "select * from xy where y-1 in (select cast(u as signed) from uv) order by 1;",
				types: []plan.JoinType{plan.JoinTypeHash},
				exp:   []sql.Row{{0, 2}, {2, 1}, {3, 3}},
			},
			{
				q:     "select * from xy where y-1 in (select u from uv order by 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemiLookup},
				exp:   []sql.Row{{0, 2}, {2, 1}, {3, 3}},
			},
			{
				q:     "select * from xy where y-1 in (select u from uv order by 1 limit 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeHash},
				exp:   []sql.Row{{2, 1}},
			},
			{
				q:     "select * from xy where x in (select u from uv join ab on u = a and a = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeRightSemiLookup, plan.JoinTypeMerge},
				exp:   []sql.Row{{2, 1}},
			},
			{
				q:     "select * from xy where x = (select u from uv join ab on u = a and a = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeRightSemiLookup, plan.JoinTypeMerge},
				exp:   []sql.Row{{2, 1}},
			},
			{
				// group by doesn't transform
				q:     "select * from xy where y-1 in (select u from uv group by v having v = 2 order by 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeHash},
				exp:   []sql.Row{{3, 3}},
			},
			{
				// window doesn't transform
				q:     "select * from xy where y-1 in (select row_number() over (order by v) from uv) order by 1;",
				types: []plan.JoinType{plan.JoinTypeHash},
				exp:   []sql.Row{{0, 2}, {3, 3}},
			},
		},
	},
	{
		name: "empty join tests",
		setup: []string{
			"CREATE table xy (x int primary key, y int);",
			"CREATE table uv (u int primary key, v int);",
			"insert into xy values (1,0), (2,1), (0,2), (3,3);",
			"insert into uv values (0,1), (1,1), (2,2), (3,2);",
		},
		tests: []JoinPlanTest{
			{
				q:     "select * from xy where y-1 = (select u from uv limit 1 offset 5);",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{},
			},
			{
				q:     "select * from xy where x != (select u from uv limit 1 offset 5);",
				types: []plan.JoinType{plan.JoinTypeAnti},
				exp:   []sql.Row{},
			},
		},
	},
	{
		name: "unnest with scope filters",
		setup: []string{
			"CREATE table xy (x int primary key, y int);",
			"CREATE table uv (u int primary key, v int);",
			"insert into xy values (1,0), (2,1), (0,2), (3,3);",
			"insert into uv values (0,1), (1,1), (2,2), (3,2);",
		},
		tests: []JoinPlanTest{
			{
				q:     "select * from xy where y-1 = (select u from uv where v = 2 order by 1 limit 1);",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{3, 3}},
			},
			{
				q:     "select * from xy where x != (select u from uv where v = 2 order by 1 limit 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeAnti},
				exp:   []sql.Row{{0, 2}, {1, 0}, {3, 3}},
			},
			{
				q:     "select * from xy where x != (select distinct u from uv where v = 2 order by 1 limit 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeAnti},
				exp:   []sql.Row{{0, 2}, {1, 0}, {3, 3}},
			},
			{
				q:     "select * from xy where (x,y+1) = (select u,v from uv where v = 2 order by 1 limit 1) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{2, 1}},
			},
			{
				q:     "select * from xy where x in (select cnt from (select count(u) as cnt from uv group by v having cnt > 0) sq) order by 1,2;",
				types: []plan.JoinType{plan.JoinTypeRightSemiLookup},
				exp:   []sql.Row{{2, 1}},
			},
			{
				q: `
SELECT * FROM xy WHERE (
      EXISTS (SELECT * FROM xy Alias1 WHERE Alias1.x = (xy.x + 1))
      AND EXISTS (SELECT * FROM uv Alias2 WHERE Alias2.u = (xy.x + 2)));`,
				types: []plan.JoinType{plan.JoinTypeSemiLookup, plan.JoinTypeMerge},
				exp:   []sql.Row{{0, 2}, {1, 0}},
			},
		},
	},
	{
		name: "unnest non-equality comparisons",
		setup: []string{
			"CREATE table xy (x int primary key, y int);",
			"CREATE table uv (u int primary key, v int);",
			"insert into xy values (1,0), (2,1), (0,2), (3,3);",
			"insert into uv values (0,1), (1,1), (2,2), (3,2);",
		},
		tests: []JoinPlanTest{
			{
				q:     "select * from xy where y >= (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{0, 2}, {3, 3}},
			},
			{
				q:     "select * from xy where x <= (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{0, 2}, {1, 0}, {2, 1}},
			},
			{
				q:     "select * from xy where x < (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{0, 2}, {1, 0}},
			},
			{
				q:     "select * from xy where x > (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{3, 3}},
			},
			{
				q:     "select * from uv where v <=> (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{2, 2}, {3, 2}},
			},
		},
	},
	{
		name: "convert semi to inner join",
		setup: []string{
			"CREATE table xy (x int, y int, primary key(x,y));",
			"CREATE table uv (u int primary key, v int);",
			"CREATE table ab (a int primary key, b int);",
			"insert into xy values (1,0), (2,1), (0,2), (3,3);",
			"insert into uv values (0,1), (1,1), (2,2), (3,2);",
			"insert into ab values (0,2), (1,2), (2,2), (3,1);",
			"update information_schema.statistics set cardinality = 100 where table_name in ('xy', 'ab', 'uv') and table_schema = 'mydb';",
		},
		tests: []JoinPlanTest{
			{
				q:     "select * from xy where x in (select u from uv join ab on u = a and a = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeHash, plan.JoinTypeMerge},
				exp:   []sql.Row{{2, 1}},
			},
			{
				q: `select x from xy where x in (
	select (select u from uv where u = sq.a)
    from (select a from ab) sq);`,
				types: []plan.JoinType{plan.JoinTypeHash},
				exp:   []sql.Row{{0}, {1}, {2}, {3}},
			},
			{
				q:     "select * from xy where y >= (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{0, 2}, {3, 3}},
			},
			{
				q:     "select * from xy where x <= (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{0, 2}, {1, 0}, {2, 1}},
			},
			{
				q:     "select * from xy where x < (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{0, 2}, {1, 0}},
			},
			{
				q:     "select * from xy where x > (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{3, 3}},
			},
			{
				q:     "select * from uv where v <=> (select u from uv where u = 2) order by 1;",
				types: []plan.JoinType{plan.JoinTypeSemi},
				exp:   []sql.Row{{2, 2}, {3, 2}},
			},
		},
	},
	{
		name: "join concat tests",
		setup: []string{
			"CREATE table xy (x int primary key, y int);",
			"CREATE table uv (u int primary key, v int);",
			"insert into xy values (1,0), (2,1), (0,2), (3,3);",
			"insert into uv values (0,1), (1,1), (2,2), (3,2);",
			"update information_schema.statistics set cardinality = 100 where table_name in ('xy', 'uv');",
		},
		tests: []JoinPlanTest{
			{
				q:     "select x, u from xy inner join uv on u+1 = x OR u+2 = x OR u+3 = x;",
				types: []plan.JoinType{plan.JoinTypeLookup},
				exp:   []sql.Row{{3, 0}, {2, 0}, {1, 0}, {3, 1}, {2, 1}, {3, 2}},
			},
		},
	},
	{
		name: "join order hint",
		setup: []string{
			"CREATE table xy (x int primary key, y int);",
			"CREATE table uv (u int primary key, v int);",
			"insert into xy values (1,0), (2,1), (0,2), (3,3);",
			"insert into uv values (0,1), (1,1), (2,2), (3,2);",
			"update information_schema.statistics set cardinality = 100 where table_name in ('xy', 'uv');",
		},
		tests: []JoinPlanTest{
			{
				q:     "select /*+ JOIN_ORDER(b, c, a) */ 1 from xy a join xy b on a.x+3 = b.x join xy c on a.x+3 = c.x and a.x+3 = b.x",
				order: []string{"b", "c", "a"},
			},
			{
				q:     "select /*+ JOIN_ORDER(a, c, b) */ 1 from xy a join xy b on a.x+3 = b.x join xy c on a.x+3 = c.x and a.x+3 = b.x",
				order: []string{"a", "c", "b"},
			},
			{
				q:     "select /*+ JOIN_ORDER(a,c,b) */ 1 from xy a join xy b on a.x+3 = b.x WHERE EXISTS (select 1 from uv c where c.u = a.x+2)",
				order: []string{"a", "c", "b"},
			},
			{
				q:     "select /*+ JOIN_ORDER(b,c,a) */ 1 from xy a join xy b on a.x+3 = b.x WHERE EXISTS (select 1 from uv c where c.u = a.x+2)",
				order: []string{"b", "c", "a"},
			},
			{
				q:     "select /*+ JOIN_ORDER(b,applySubq0,a) */ 1 from xy a join xy b on a.x+3 = b.x WHERE a.x in (select u from uv c)",
				order: []string{"b", "applySubq0", "a"},
			},
		},
	},
}

func TestJoinPlanning(t *testing.T, harness Harness) {
	for _, tt := range JoinPlanningTests {
		t.Run(tt.name, func(t *testing.T) {
			harness.Setup([]setup.SetupScript{setup.MydbData[0], tt.setup})
			e := mustNewEngine(t, harness)
			defer e.Close()
			for _, tt := range tt.tests {
				if tt.types != nil {
					evalJoinTypeTest(t, harness, e, tt)
				}
				if tt.exp != nil {
					evalJoinCorrectness(t, harness, e, tt.q, tt.q, tt.exp, tt.skip)
				}
				if tt.order != nil {
					evalJoinOrder(t, harness, e, tt.q, tt.order, tt.skip)
				}
			}
		})
	}
}

func evalJoinTypeTest(t *testing.T, harness Harness, e *sqle.Engine, tt JoinPlanTest) {
	t.Run(tt.q+" join types", func(t *testing.T) {
		if tt.skip {
			t.Skip()
		}

		ctx := NewContext(harness)
		ctx = ctx.WithQuery(tt.q)

		a, err := e.AnalyzeQuery(ctx, tt.q)
		require.NoError(t, err)

		jts := collectJoinTypes(a)
		var exp []string
		for _, t := range tt.types {
			exp = append(exp, t.String())
		}
		var cmp []string
		for _, t := range jts {
			cmp = append(cmp, t.String())
		}
		require.Equal(t, exp, cmp, fmt.Sprintf("unexpected plan:\n%s", sql.DebugString(a)))
	})
}

func evalJoinCorrectness(t *testing.T, harness Harness, e *sqle.Engine, name, q string, exp []sql.Row, skip bool) {
	t.Run(name, func(t *testing.T) {
		if skip {
			t.Skip()
		}

		ctx := NewContext(harness)
		ctx = ctx.WithQuery(q)

		sch, iter, err := e.QueryWithBindings(ctx, q, nil)
		require.NoError(t, err, "Unexpected error for query %s: %s", q, err)

		rows, err := sql.RowIterToRows(ctx, sch, iter)
		require.NoError(t, err, "Unexpected error for query %s: %s", q, err)

		if exp != nil {
			checkResults(t, exp, nil, sch, rows, q)
		}

		require.Equal(t, 0, ctx.Memory.NumCaches())
		validateEngine(t, ctx, harness, e)
	})
}

func collectJoinTypes(n sql.Node) []plan.JoinType {
	var types []plan.JoinType
	transform.Inspect(n, func(n sql.Node) bool {
		if n == nil {
			return true
		}
		j, ok := n.(*plan.JoinNode)
		if ok {
			types = append(types, j.Op)
		}

		if ex, ok := n.(sql.Expressioner); ok {
			for _, e := range ex.Expressions() {
				transform.InspectExpr(e, func(e sql.Expression) bool {
					sq, ok := e.(*plan.Subquery)
					if !ok {
						return false
					}
					types = append(types, collectJoinTypes(sq.Query)...)
					return false
				})
			}
		}
		return true
	})
	return types
}

func evalJoinOrder(t *testing.T, harness Harness, e *sqle.Engine, q string, exp []string, skip bool) {
	t.Run(q+" join order", func(t *testing.T) {
		if skip {
			t.Skip()
		}

		ctx := NewContext(harness)
		ctx = ctx.WithQuery(q)

		a, err := e.AnalyzeQuery(ctx, q)
		require.NoError(t, err)

		cmp := collectJoinOrder(a)
		require.Equal(t, exp, cmp, fmt.Sprintf("expected order '%s' found '%s'\ndetail:\n%s", strings.Join(exp, ","), strings.Join(cmp, ","), sql.DebugString(a)))
	})
}

func collectJoinOrder(n sql.Node) []string {
	var order []string

	j, ok := n.(*plan.JoinNode)
	if ok {
		if n, ok := j.Left().(sql.NameableNode); ok {
			order = append(order, n.Name())
		}
	}
	for _, n := range n.Children() {
		order = append(order, collectJoinOrder(n)...)
	}
	if ok {
		switch n := j.Right().(type) {
		case sql.NameableNode:
			order = append(order, n.Name())
		case *plan.HashLookup:
			ok = false
			r := n.Child
			for !ok {
				switch n := r.(type) {
				case sql.NameableNode:
					order = append(order, n.Name())
					ok = true
				case *plan.JoinNode:
					ok = true
				case *plan.Distinct:
					r = n.Child
				case *plan.Project:
					r = n.Child
				case *plan.CachedResults:
					r = n.Child
				default:
					ok = true
				}
			}
		}
	}

	if ex, ok := n.(sql.Expressioner); ok {
		for _, e := range ex.Expressions() {
			transform.InspectExpr(e, func(e sql.Expression) bool {
				sq, ok := e.(*plan.Subquery)
				if !ok {
					return false
				}
				order = append(order, collectJoinOrder(sq.Query)...)
				return false
			})
		}
	}
	return order
}

func TestJoinPlanningPrepared(t *testing.T, harness Harness) {
	for _, tt := range JoinPlanningTests {
		t.Run(tt.name, func(t *testing.T) {
			harness.Setup([]setup.SetupScript{setup.MydbData[0], tt.setup})
			e := mustNewEngine(t, harness)
			defer e.Close()
			for _, tt := range tt.tests {
				if tt.types != nil {
					evalJoinTypeTestPrepared(t, harness, e, tt)
				}
				if tt.exp != nil {
					evalJoinCorrectnessPrepared(t, harness, e, tt.q, tt.q, tt.exp, tt.skip)
				}
				if tt.order != nil {
					evalJoinOrderPrepared(t, harness, e, tt.q, tt.order, tt.skip)
				}
			}
		})
	}
}

func evalJoinTypeTestPrepared(t *testing.T, harness Harness, e *sqle.Engine, tt JoinPlanTest) {
	t.Run(tt.q+" join types", func(t *testing.T) {
		if tt.skip {
			t.Skip()
		}

		ctx := NewContext(harness)
		ctx = ctx.WithQuery(tt.q)

		bindings, err := injectBindVarsAndPrepare(t, ctx, e, tt.q)
		require.NoError(t, err)

		p, ok := e.PreparedDataCache.GetCachedStmt(ctx.Session.ID(), tt.q)
		require.True(t, ok, "prepared statement not found")

		if len(bindings) > 0 {
			var usedBindings map[string]bool
			p, usedBindings, err = plan.ApplyBindings(p, bindings)
			require.NoError(t, err)
			for binding := range bindings {
				require.True(t, usedBindings[binding], "unused binding %s", binding)
			}
		}

		a, _, err := e.Analyzer.AnalyzePrepared(ctx, p, nil)
		require.NoError(t, err)

		jts := collectJoinTypes(a)
		require.Equal(t, tt.types, jts)
	})
}

func evalJoinCorrectnessPrepared(t *testing.T, harness Harness, e *sqle.Engine, name, q string, exp []sql.Row, skip bool) {
	t.Run(q, func(t *testing.T) {
		if skip {
			t.Skip()
		}

		ctx := NewContext(harness)
		ctx = ctx.WithQuery(q)

		bindings, err := injectBindVarsAndPrepare(t, ctx, e, q)
		require.NoError(t, err)

		sch, iter, err := e.QueryWithBindings(ctx, q, bindings)
		require.NoError(t, err, "Unexpected error for query %s: %s", q, err)

		rows, err := sql.RowIterToRows(ctx, sch, iter)
		require.NoError(t, err, "Unexpected error for query %s: %s", q, err)

		if exp != nil {
			checkResults(t, exp, nil, sch, rows, q)
		}

		require.Equal(t, 0, ctx.Memory.NumCaches())
		validateEngine(t, ctx, harness, e)
	})
}

func evalJoinOrderPrepared(t *testing.T, harness Harness, e *sqle.Engine, q string, exp []string, skip bool) {
	t.Run(q+" join order", func(t *testing.T) {
		if skip {
			t.Skip()
		}

		ctx := NewContext(harness)
		ctx = ctx.WithQuery(q)

		bindings, err := injectBindVarsAndPrepare(t, ctx, e, q)
		require.NoError(t, err)

		p, ok := e.PreparedDataCache.GetCachedStmt(ctx.Session.ID(), q)
		require.True(t, ok, "prepared statement not found")

		if len(bindings) > 0 {
			var usedBindings map[string]bool
			p, usedBindings, err = plan.ApplyBindings(p, bindings)
			require.NoError(t, err)
			for binding := range bindings {
				require.True(t, usedBindings[binding], "unused binding %s", binding)
			}
		}

		a, _, err := e.Analyzer.AnalyzePrepared(ctx, p, nil)
		require.NoError(t, err)

		cmp := collectJoinOrder(a)
		require.Equal(t, exp, cmp, fmt.Sprintf("expected order '%s' found '%s'\ndetail:\n%s", strings.Join(exp, ","), strings.Join(cmp, ","), sql.DebugString(a)))
	})
}
