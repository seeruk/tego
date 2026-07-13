package tego

import (
	"strings"

	"github.com/danielgtaylor/casing"
	"github.com/seeruk/tego/tegopb"
	"google.golang.org/protobuf/compiler/protogen"
)

func (p *Planner) planEnum(enum *ProtoEnum) (EnumPlan, []Diagnostic, bool) {
	if enum.Options.GetOmit() || enum.Options.HasGoType() {
		return EnumPlan{}, nil, false
	}

	underlying, diagnostics := enumUnderlyingType(enum)
	name := plannedEnumName(enum)

	plan := EnumPlan{
		ProtoName:  enum.FullName,
		Name:       name,
		Underlying: underlying,
		Comment: plannedComment(
			enum.Options.GetComment(),
			enum.Options.HasComment(),
			enumLeadingComment(enum),
			string(enum.Name),
			name,
		),
	}

	for _, value := range enum.Values {
		constant, constantDiagnostics, ok := p.planEnumConstant(value, underlying, plan.Name)
		diagnostics = append(diagnostics, constantDiagnostics...)
		if ok {
			plan.Constants = append(plan.Constants, constant)
		}
	}

	return plan, diagnostics, true
}

func enumUnderlyingType(enum *ProtoEnum) (EnumUnderlyingType, []Diagnostic) {
	if !enum.Options.HasUnderlyingType() {
		return EnumUnderlyingTypeUint, nil
	}

	switch enum.Options.GetUnderlyingType() {
	case tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_UINT:
		return EnumUnderlyingTypeUint, nil
	case tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_INT:
		return EnumUnderlyingTypeInt, nil
	case tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_STRING:
		return EnumUnderlyingTypeString, nil
	default:
		return EnumUnderlyingTypeUint, []Diagnostic{fatalDiagnostic(
			string(enum.FullName),
			"unsupported enum underlying type %s",
			enum.Options.GetUnderlyingType(),
		)}
	}
}

func (p *Planner) planEnumConstant(value *ProtoEnumValue, underlying EnumUnderlyingType, enumName string) (EnumConstantPlan, []Diagnostic, bool) {
	if value.Options.GetOmit() {
		return EnumConstantPlan{}, nil, false
	}

	name := plannedEnumConstantName(value, enumName)

	plan := EnumConstantPlan{
		ProtoName: value.FullName,
		Name:      name,
		Comment: plannedComment(
			value.Options.GetComment(),
			value.Options.HasComment(),
			enumValueLeadingComment(value),
			string(value.Name),
			name,
		),
	}

	diagnostics := enumConstantValue(value, underlying, enumName, plan.Name, &plan.Value)

	return plan, diagnostics, true
}

func enumConstantValue(value *ProtoEnumValue, underlying EnumUnderlyingType, enumName, plannedName string, out *EnumConstantValue) []Diagnostic {
	switch underlying {
	case EnumUnderlyingTypeUint:
		if value.Options.HasValue() && !value.Options.HasUint() {
			return []Diagnostic{enumValueMismatchDiagnostic(value, "uint")}
		}
		if value.Options.HasUint() {
			out.Uint = uint(value.Options.GetUint())
		} else {
			out.Uint = uint(value.Number)
		}
	case EnumUnderlyingTypeInt:
		if value.Options.HasValue() && !value.Options.HasInt() {
			return []Diagnostic{enumValueMismatchDiagnostic(value, "int")}
		}
		if value.Options.HasInt() {
			out.Int = int(value.Options.GetInt())
		} else {
			out.Int = int(value.Number)
		}
	case EnumUnderlyingTypeString:
		if value.Options.HasValue() && !value.Options.HasString() {
			return []Diagnostic{enumValueMismatchDiagnostic(value, "string")}
		}
		if value.Options.HasString() {
			out.String = value.Options.GetString()
		} else {
			out.String = casingScreamingSnake(strings.TrimPrefix(plannedName, enumName))
		}
	}

	return nil
}

func enumValueMismatchDiagnostic(value *ProtoEnumValue, underlying string) Diagnostic {
	return fatalDiagnostic(
		string(value.FullName),
		"enum value override must match %s underlying type",
		underlying,
	)
}

func enumLeadingComment(enum *ProtoEnum) protogen.Comments {
	if enum.Desc == nil {
		return ""
	}
	return enum.Desc.Comments.Leading
}

func enumValueLeadingComment(value *ProtoEnumValue) protogen.Comments {
	if value.Desc == nil {
		return ""
	}
	return value.Desc.Comments.Leading
}

func casingScreamingSnake(value string, transforms ...casing.TransformFunc) string {
	transforms = append(transforms, strings.ToUpper)
	return casing.Join(casing.MergeNumbers(casing.Split(value)), "_", transforms...)
}
