/**
 * ast_to_sp.go
 * 
 * Copyright 2020 Nirari Technologies.
 * 
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
 * 
 * The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 * 
 */

package GoToSPGen


import (
	"strings"
	"fmt"
	"unicode"
	//"bytes"
	"go/token"
	"go/ast"
	"go/types"
	//"go/format"
	//"go/constant"
	"../ast_transform"
)

var (
	FuncNames = map[string]string{
		"len":  "sizeof",
		"main": "OnPluginStart",
	}
	
	IdenNames = map[string]string{
		"nil":  "null",
	}
)

const (
	Header string = "/**\n * file generated by the GoToSourcePawn Transpiler v1.2b\n * Copyright 2020 (C) Kevin Yonan aka Nergal, Assyrianic.\n * GoToSourcePawn Project is licensed under MIT.\n * link: 'https://github.com/assyrianic/Go2SourcePawn'\n */\n\n"
	
	GENFLAG_NEWLINE = 1
	GENFLAG_SEMICOLON = 2
)

func InsertStr(a []string, index int, value string) []string {
	if len(a) <= index { /// nil or empty slice or after last element
		return append(a, value)
	}
	a = append(a[:index+1], a[index:]...) /// index < len(a)
	a[index] = value
	return a
}

func WriteTabStr(count uint) string {
	var s string
	for i:=uint(0); i<count; i++ {
		s += "\t"
	}
	return s
}

type (
	FuncBlock struct {
		Body strings.Builder
		Tabs uint
		Params []string
		Storage, RetType, Name string
	}
	
	EStruct struct {
		Methods []FuncBlock
		Fields []string
	}
	
	MethodMap struct {
		Methods, Props []FuncBlock
		Name string
	}
	
	SMPlugin struct {
		Includes, Globals []string
		Structs map[string]EStruct
		//MethodMaps []MethodMap
		Funcs []FuncBlock
	}
)


func ReplaceName(s *string, typ, rep string) bool {
	if strings.Contains(*s, typ) {
		*s = strings.Replace(*s, typ, rep, -1)
		return true
	}
	return false
}

type TypeString struct {
	TypeName, LhsBracks, Name, RhsBracks string
}

func (ts TypeString) Join(param, is_ref, is_array bool) string {
	var total strings.Builder
	if param && is_array && !is_ref {
		total.WriteString("const ")
	}
	
	total.WriteString(ts.TypeName)
	
	if is_array {
		total.WriteString(ts.LhsBracks)
	} else if is_ref {
		total.WriteString("&")
	}
	
	if len(ts.Name) > 0 {
		total.WriteString(" " + ts.Name)
	}
	
	if is_array {
		total.WriteString(ts.RhsBracks)
	}
	return total.String()
}

/**
 * SP params -> [const] TypeName ([]... | &) VarName ([N]...)
 * SP vars   ->         TypeName VarName ([(N)]...)
 * SP ret    ->         TypeName ([])
 */
func GetTypeString(expr ast.Expr, name string, param bool) string {
	ts := TypeString{Name: name}
	var is_ref, is_array bool
	if typ := ASTMod.ASTCtxt.TypeInfo.TypeOf(expr); typ != nil {
	recheck:
		original := typ.String()
		type_name := strings.Replace(original, "untyped ", "", -1)
		if strings.HasPrefix(type_name, "func(") {
			ts.TypeName = "Function"
		} else if type_name=="invalid type" {
			ts.TypeName = "any"
		} else {
			//if n, found := TypeNames[type_name]; found {
			//	type_name = n
			//}
			type_name = strings.TrimFunc(type_name, func(r rune) bool {
				return !unicode.IsLetter(r)
			})
			ts.TypeName = type_name
		}
		
		switch t := typ.(type) {
			case *types.Array:
				//fmt.Printf("Array::type_name: %s\n", type_name)
				ts.RhsBracks += fmt.Sprintf("[%d]", t.Len())
				is_array = true
				typ = t.Elem()
				goto recheck
			case *types.Named:
				//fmt.Printf("Named::type_name: %s\n", type_name)
				if _, is_struct := t.Underlying().(*types.Struct); is_struct {
					is_array = true
				} /*else {
					typ = t.Underlying()
					goto recheck
				}*/
			case *types.Map:
				ts.TypeName = "StringMap"
			case *types.Slice:
				//fmt.Printf("Slice::type_name: %s\n", type_name)
				typ = t.Elem()
				if param {
					ts.LhsBracks += "[]"
				} else {
					ts.RhsBracks += "[]"
				}
				is_array = true
				goto recheck
			case *types.Pointer:
				//fmt.Printf("Pointer::type_name: %s\n", type_name)
				if !is_ref {
					is_ref = true
				} else {
					ASTMod.PrintSrcGoErr(expr.Pos(), "Multi-Pointers are Illegal.")
				}
				typ = t.Elem()
				goto recheck
			case *types.Basic:
				//fmt.Printf("Basic::type_name: %s\n", type_name)
				if type_name=="string" {
					ts.TypeName = "char"
					if param {
						ts.LhsBracks += "[]"
					} else {
						ts.RhsBracks += "[]"
					}
					is_array = true
				}
		}
	}
	//fmt.Printf("\n")
	return ts.Join(param, is_ref, is_array)
}

func IsSameType(a ast.Expr, b []ast.Expr, param bool) bool {
	typeA := GetTypeString(a, "", param)
	for _, c := range b {
		if typeA != GetTypeString(c, "", param) {
			return false
		}
	}
	return true
}


func WriteParams(flist *ast.FieldList) []string {
	param_list := make([]string, 0)
	for _, parm := range flist.List {
		for _, name := range parm.Names {
			param_list = append(param_list, GetTypeString(parm.Type, name.Name, true))
		}
	}
	return param_list
}

func WriteStructMembs(flist *ast.FieldList) []string {
	field_list := make([]string, 0)
	for _, field := range flist.List {
		for _, member_name := range field.Names {
			field_str := GetTypeString(field.Type, member_name.Name, false)
			field_list = append(field_list, field_str)
		}
	}
	return field_list
}


func GeneratePluginFile(file *ast.File) string {
	plugin := SMPlugin{Structs: make(map[string]EStruct)}
	/// read imports.
	ast.Inspect(file, func(n ast.Node) bool {
		if n != nil {
			if imp, is_import := n.(*ast.ImportSpec); is_import {
				if imp.Path.Value[1] == '.' {
					plugin.Includes = append(plugin.Includes, `#include "` + imp.Path.Value[2:] + `"`)
				} else {
					plugin.Includes = append(plugin.Includes, "#include <" + imp.Path.Value[1 : len(imp.Path.Value)-1] + ">")
				}
			}
		}
		return true
	})
	
	ast.Inspect(file, func(n ast.Node) bool {
		if n != nil {
			if decl, is_gendecl := n.(*ast.GenDecl); is_gendecl {
				switch decl.Tok {
					case token.CONST:
						for _, spec := range decl.Specs {
							plugin.Globals = append(plugin.Globals, MakeConstSpec(spec.(*ast.ValueSpec), 0))
						}
						plugin.Globals = append(plugin.Globals, "\n")
					
					case token.TYPE:
						for _, spec := range decl.Specs {
							plugin.MakeTypeSpec(spec.(*ast.TypeSpec))
						}
				}
			}
		}
		return true
	})
	
	for _, d := range file.Decls {
		switch decl := d.(type) {
			case *ast.GenDecl:
				switch decl.Tok {
					case token.VAR:
						for _, spec := range decl.Specs {
							plugin.Globals = append(plugin.Globals, MakeVarSpec(spec.(*ast.ValueSpec), 0))
						}
				}
			case *ast.FuncDecl:
				plugin.MakeFuncDecl(decl)
		}
	}
	
	var plugin_src_code strings.Builder
	plugin_src_code.WriteString(Header)
	for _, inc := range plugin.Includes {
		plugin_src_code.WriteString(inc + "\n")
	}
	plugin_src_code.WriteString("\n")
	for _, global := range plugin.Globals {
		plugin_src_code.WriteString(global + "\n")
	}
	
	single_tab := WriteTabStr(1)
	for name, struc := range plugin.Structs {
		plugin_src_code.WriteString(fmt.Sprintf("enum struct %s {", name))
		for _, field := range struc.Fields {
			plugin_src_code.WriteString("\n" + single_tab + field + ";")
		}
		if struc.Methods != nil {
			plugin_src_code.WriteString("\n\n")
			for i, method := range struc.Methods {
				plugin_src_code.WriteString(single_tab + method.RetType + " " + method.Name + "(")
				for index, param := range method.Params {
					plugin_src_code.WriteString(param)
					if index+1 != len(method.Params) {
						plugin_src_code.WriteString(", ")
					}
				}
				plugin_src_code.WriteString(")" + method.Body.String())
				if i+1 != len(struc.Methods) {
					plugin_src_code.WriteString("\n")
				}
			}
		}
		plugin_src_code.WriteString("\n}\n\n")
	}
	
	for i, fn := range plugin.Funcs {
		plugin_src_code.WriteString(fn.Storage + " " + fn.RetType + " " + fn.Name + "(")
		for index, param := range fn.Params {
			plugin_src_code.WriteString(param)
			if index+1 != len(fn.Params) {
				plugin_src_code.WriteString(", ")
			}
		}
		plugin_src_code.WriteString(")")
		plugin_src_code.WriteString(fn.Body.String())
		if i+1 != len(plugin.Funcs) {
			plugin_src_code.WriteString("\n\n")
		}
	}
	return plugin_src_code.String()
}

func MakeConstSpec(const_spec *ast.ValueSpec, tabs uint) string {
	/// if a constant is untyped, it can have different names and associating values.
	var const_str strings.Builder
	tabstrone := WriteTabStr(tabs + 1)
	tabstr := WriteTabStr(tabs)
	if const_spec.Type != nil {
		for _, name := range const_spec.Names {
			for _, value := range const_spec.Values {
				switch val := value.(type) {
					case *ast.CompositeLit:
						const_str.WriteString(tabstr + GetTypeString(const_spec.Type, name.Name, false) + " = {")
						for n, expr := range val.Elts {
							const_str.WriteString("\n" + tabstrone + GetExprString(expr))
							if n+1 != len(val.Elts) {
								const_str.WriteString(",")
							}
						}
						const_str.WriteString(tabstr + "};")
						
					default:
						const_str.WriteString(GetTypeString(const_spec.Type, name.Name, false) + " = " + GetExprString(value) + ";")
				}
				const_str.WriteString("\n")
			}
		}
	} else {
		for i, name := range const_spec.Names {
			switch val := const_spec.Values[i].(type) {
				case *ast.CompositeLit:
					const_str.WriteString(tabstr + GetTypeString(val.Type, name.Name, false) + " = {")
					for n, expr := range val.Elts {
						const_str.WriteString("\n" + tabstrone + GetExprString(expr))
						if n+1 != len(val.Elts) {
							const_str.WriteString(",")
						}
					}
					const_str.WriteString(tabstr + "};")
					
				default:
					const_str.WriteString(GetTypeString(const_spec.Values[i], name.Name, false) + " = " + GetExprString(val) + ";")
			}
			const_str.WriteString("\n")
		}
	}
	return const_str.String()
}


func MakeVarSpec(var_spec *ast.ValueSpec, tabs uint) string {
	/// if a constant is untyped, it can have different names and associating values.
	tabstrone := WriteTabStr(tabs + 1)
	tabstr := WriteTabStr(tabs)
	var var_str strings.Builder
	if var_spec.Type != nil {
		for i, name := range var_spec.Names {
			var_str.WriteString(tabstr + GetTypeString(var_spec.Type, name.Name, false))
			if var_spec.Values != nil && i < len(var_spec.Values) {
				switch val := var_spec.Values[i].(type) {
					case *ast.CompositeLit:
						var_str.WriteString(" = {")
						for n, expr := range val.Elts {
							var_str.WriteString(tabstrone + GetExprString(expr))
							if n+1 != len(val.Elts) {
								var_str.WriteString(",")
							}
							var_str.WriteString("\n")
						}
						var_str.WriteString(tabstr + "}")
					default:
						var_str.WriteString(" = " + GetExprString(var_spec.Values[i]))
				}
			}
			var_str.WriteString(";\n")
		}
	} else {
		for _, name := range var_spec.Names {
			for _, value := range var_spec.Values {
				var_str.WriteString(tabstr + GetTypeString(value, name.Name, false))
				switch val := value.(type) {
					case *ast.CompositeLit:
						var_str.WriteString(" = {\n")
						for n, expr := range val.Elts {
							var_str.WriteString(tabstrone + GetExprString(expr))
							if n+1 != len(val.Elts) {
								var_str.WriteString(",")
							}
							var_str.WriteString("\n")
						}
						var_str.WriteString(tabstr + "}")
					default:
						var_str.WriteString(" = " + GetExprString(value))
				}
			}
			var_str.WriteString(";\n")
		}
	}
	return var_str.String()
}

func (plugin *SMPlugin) MakeTypeSpec(type_spec *ast.TypeSpec) {
	switch t := type_spec.Type.(type) {
		case *ast.StructType:
			plugin.Structs[type_spec.Name.Name] = EStruct{
				Fields:  WriteStructMembs(t.Fields),
			}
		
		case *ast.FuncType:
			var func_type strings.Builder
			/// typedef Whatever = function type (params);
			func_type.WriteString("typedef " + type_spec.Name.Name + " = function ")
			if t.Results != nil {
				type_str := GetTypeString(t.Results.List[0].Type, "", false)
				type_str = strings.TrimSpace(type_str)
				if strings.Count(type_str, "[") > 0 {
					/// shoot error but continue.
					ASTMod.PrintSrcGoErr(t.Pos(), "Typedef'd functions are not allowed to return arrays.")
				}
				func_type.WriteString(type_str)
			} else {
				func_type.WriteString("void")
			}
			func_type.WriteString(" (")
			
			param_list := WriteParams(t.Params)
			param_count := len(param_list)
			for i, param := range param_list {
				func_type.WriteString(param)
				if i+1 != param_count {
					func_type.WriteString(", ")
				}
			}
			func_type.WriteString(");")
			plugin.Globals = append(plugin.Globals, func_type.String() + "\n")
	}
}

func (plugin *SMPlugin) MakeFuncDecl(f *ast.FuncDecl) {
	fn := FuncBlock{}
	if f.Type.Results != nil {
		fn.RetType = GetTypeString(f.Type.Results.List[0].Type, "", false)
	} else {
		fn.RetType = "void"
	}
	
	if replace_name, found := FuncNames[f.Name.Name]; found {
		f.Name.Name = replace_name
	}
	
	fn.Name = f.Name.Name
	fn.Params = WriteParams(f.Type.Params)
	
	if f.Recv != nil {
		fn.Tabs = 1
	} else {
		fn.Tabs = 0
	}
	
	if f.Body != nil {
		fn.Storage = "public"
		fn.MakeStmts(f.Body.List, GENFLAG_NEWLINE | GENFLAG_SEMICOLON)
	} else {
		fn.Storage = "native"
		fn.Body.WriteString(";")
	}
	
	if f.Recv != nil {
		struct_type := GetTypeString(f.Recv.List[0].Type, "", false)
		if struc, ok := plugin.Structs[struct_type]; ok {
			struc.Methods = append(struc.Methods, fn)
			plugin.Structs[struct_type] = struc
		}
	} else {
		plugin.Funcs = append(plugin.Funcs, fn)
	}
}


func (cb *FuncBlock) MakeStmts(stmts []ast.Stmt, flags int) {
	tabstr := WriteTabStr(cb.Tabs)
	cb.Body.WriteString("\n" + tabstr + "{")
	cb.Tabs++
	for _, stmt := range stmts {
		cb.MakeStmt(stmt, flags)
	}
	cb.Tabs--
	cb.Body.WriteString("\n" + tabstr + "}")
}


func (cb *FuncBlock) MakeStmt(stmt ast.Stmt, flags int) {
	if flags & GENFLAG_NEWLINE > 0 {
		cb.Body.WriteString("\n")
	}
	tabstr := WriteTabStr(cb.Tabs)
	switch n := stmt.(type) {
		case *ast.BlockStmt:
			cb.MakeStmts(n.List, flags)
		
		case *ast.AssignStmt:
			left_len, rite_len := len(n.Lhs), len(n.Rhs)
			if n.Tok==token.DEFINE {
				/// TODO: make this more robust.
				for i,e := range n.Lhs {
					var_name := e.(*ast.Ident)
					cb.Body.WriteString(tabstr + GetTypeString(n.Lhs[i], var_name.Name, false) + " = " + GetExprString(n.Rhs[i]))
					if flags & GENFLAG_SEMICOLON > 0 {
						cb.Body.WriteString(";")
					}
					if i+1 != left_len {
						if flags & GENFLAG_NEWLINE > 0 {
							cb.Body.WriteString("\n")
						}
					}
				}
			} else {
				if left_len==rite_len {
					for i := range n.Lhs {
						cb.Body.WriteString(tabstr + GetExprString(n.Lhs[i]) + " " + n.Tok.String() + " " + GetExprString(n.Rhs[i]))
						if flags & GENFLAG_SEMICOLON > 0 {
							cb.Body.WriteString(";")
						}
						if i+1 != left_len {
							cb.Body.WriteString("\n")
						}
					}
				} else if rite_len==1 && left_len >= rite_len {
					for i := range n.Lhs {
						cb.Body.WriteString(tabstr + GetExprString(n.Lhs[i]) + " " + n.Tok.String() + " " + GetExprString(n.Rhs[0]))
						if flags & GENFLAG_SEMICOLON > 0 {
							cb.Body.WriteString(";")
						}
						if i+1 != left_len {
							cb.Body.WriteString("\n")
						}
					}
				}
			}
			return
		
		case *ast.ExprStmt:
			if fn_call, is_call := n.X.(*ast.CallExpr); is_call {
				if name := GetExprString(fn_call.Fun); name == "__sp__" {
					if len(fn_call.Args) > 0 {
						code_value := fn_call.Args[0].(*ast.BasicLit).Value
						cb.Body.WriteString(tabstr + code_value[1 : len(code_value)-1])
						return
					}
				}
			}
			cb.Body.WriteString(tabstr + GetExprString(n.X))
		
		case *ast.IncDecStmt:
			cb.Body.WriteString(tabstr + GetExprString(n.X) + n.Tok.String())
		
		case *ast.ReturnStmt:
			cb.Body.WriteString(tabstr + "return")
			if n.Results != nil {
				cb.Body.WriteString(" " + GetExprString(n.Results[0]))
			}
			
		case *ast.DeclStmt:
			gd := n.Decl.(*ast.GenDecl)
			switch gd.Tok {
				case token.VAR:
					for _, spec := range gd.Specs {
						cb.Body.WriteString(MakeVarSpec(spec.(*ast.ValueSpec), cb.Tabs))
					}
			}
			return
		
		case *ast.ForStmt:
			cb.Body.WriteString(tabstr + "for (")
			if n.Init != nil {
				old := cb.Tabs
				cb.Tabs = 0
				cb.MakeStmt(n.Init, 0)
				cb.Tabs = old
			}
			cb.Body.WriteString(";")
			if n.Cond != nil { /// condition; or nil
				cb.Body.WriteString(" ")
				cb.Body.WriteString(GetExprString(n.Cond))
			}
			cb.Body.WriteString(";")
			if n.Post != nil { /// post iteration statement; or nil
				cb.Body.WriteString(" ")
				old := cb.Tabs
				cb.Tabs = 0
				cb.MakeStmt(n.Post, 0)
				cb.Tabs = old
			}
			cb.Body.WriteString(")")
			cb.MakeStmts(n.Body.List, GENFLAG_NEWLINE | GENFLAG_SEMICOLON)
			return
		
		case *ast.IfStmt:
			if_stmt := n
			cb.Body.WriteString(tabstr)
		re_if:
			cb.Body.WriteString("if (" + GetExprString(if_stmt.Cond) + ")")
			cb.MakeStmts(if_stmt.Body.List, GENFLAG_NEWLINE | GENFLAG_SEMICOLON)
			if if_stmt.Else != nil {
				cb.Body.WriteString("\n" + tabstr + "else ")
				switch s := if_stmt.Else.(type) {
					case *ast.IfStmt:
						if_stmt = s
						goto re_if
					default:
						else_block := s.(*ast.BlockStmt)
						cb.MakeStmts(else_block.List, GENFLAG_NEWLINE)
				}
			}
			return
			
		case *ast.BranchStmt:
			cb.Body.WriteString(tabstr + n.Tok.String())
		
		/// for a,b := range array {}
		/// for (int a; a < sizeof(array); a++) { type b = array[a]; }
		case *ast.RangeStmt:
			key_str := GetExprString(n.Key)
			cb.Body.WriteString(tabstr + fmt.Sprintf("for (int %s; %s < sizeof(%s); %s++)", key_str, key_str, GetExprString(n.X), key_str))
			cb.MakeStmts(n.Body.List, GENFLAG_NEWLINE | GENFLAG_SEMICOLON)
			return
		
		case *ast.SwitchStmt:
			/// if no tag expression, make it an if-else-if series.
			if n.Tag != nil {
				cb.Body.WriteString(tabstr + "switch (" + GetExprString(n.Tag) + ")")
				cb.MakeStmts(n.Body.List, GENFLAG_NEWLINE | GENFLAG_SEMICOLON)
			} else {
				var case_list []*ast.CaseClause
				for _, stmt := range n.Body.List {
					case_list = append(case_list, stmt.(*ast.CaseClause))
				}
				cases := len(case_list)
				for i, case_ := range case_list {
					var put_rite_paren bool
					if i==0 {
						cb.Body.WriteString(tabstr + "if (")
						put_rite_paren = true
					} else if i+1 != cases {
						cb.Body.WriteString(tabstr + "else if (")
						put_rite_paren = true
					} else {
						cb.Body.WriteString(tabstr + "else")
					}
					
					conds := len(case_.List)
					if conds==0 && i==0 {
						cb.Body.WriteString("true")
					} else {
						for n, expr := range case_.List {
							cb.Body.WriteString("(" + GetExprString(expr) + ")")
							if n+1 != conds {
								cb.Body.WriteString(" || ")
							}
						}
					}
					if put_rite_paren {
						cb.Body.WriteString(")")
					}
					cb.MakeStmts(case_.Body, GENFLAG_NEWLINE | GENFLAG_SEMICOLON)
					if i+1 != cases {
						cb.Body.WriteString("\n")
					}
				}
			}
			return
		
		case *ast.CaseClause:
			cases := len(n.List)
			if cases==0 {
				cb.Body.WriteString(tabstr + "default:")
				cb.MakeStmts(n.Body, GENFLAG_NEWLINE | GENFLAG_SEMICOLON)
			} else {
				cb.Body.WriteString(tabstr + "case ")
				for i, expr := range n.List {
					cb.Body.WriteString(GetExprString(expr))
					if i+1 != cases {
						cb.Body.WriteString(", ")
					}
				}
				cb.Body.WriteString(":")
				cb.MakeStmts(n.Body, GENFLAG_NEWLINE | GENFLAG_SEMICOLON)
			}
			return
		
		case *ast.EmptyStmt, *ast.CommClause, *ast.DeferStmt, *ast.TypeSwitchStmt, *ast.LabeledStmt, *ast.GoStmt, *ast.SelectStmt, *ast.SendStmt:
			cb.Body.WriteString(tabstr + "/// illegal operation: " + ASTMod.PrettyPrintAST(stmt))
	}
	
	if flags & GENFLAG_SEMICOLON > 0 {
		cb.Body.WriteString(";")
	}
}

func GetExprString(e ast.Expr) string {
	switch x := e.(type) {
		case *ast.IndexExpr:
			return GetExprString(x.X) + "[" + GetExprString(x.Index) + "]"
		
		case *ast.KeyValueExpr:
			return GetExprString(x.Key) + " = " + GetExprString(x.Value)
		
		case *ast.ParenExpr:
			return "(" + GetExprString(x.X) + ")"
		
		case *ast.StarExpr:
			/// ignore deref star
			return GetExprString(x.X)
		
		case *ast.UnaryExpr:
			if x.Op==token.AND {
				return GetExprString(x.X)
			} else if x.Op==token.XOR {
				return "~" + GetExprString(x.X)
			}
			return x.Op.String() + GetExprString(x.X)
		
		case *ast.CallExpr:
			var call strings.Builder
			name := GetExprString(x.Fun)
			if n, found := FuncNames[name]; found {
				name = n
			}
			call.WriteString(name + "(")
			for i, arg := range x.Args {
				call.WriteString(GetExprString(arg))
				if i+1 != len(x.Args) {
					call.WriteString(", ")
				}
			}
			call.WriteString(")")
			return call.String()
		
		case *ast.BinaryExpr:
			return GetExprString(x.X) + " " + x.Op.String() + " " + GetExprString(x.Y)
		
		case *ast.SelectorExpr:
			return GetExprString(x.X) + "." + GetExprString(x.Sel)
		
		case *ast.Ident:
			if n, found := IdenNames[x.Name]; found {
				return n
			}
			return x.Name
		
		case *ast.BasicLit:
			return x.Value
		
		case *ast.TypeAssertExpr, *ast.SliceExpr:
			return ""
	}
	return ""
}