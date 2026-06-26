package tego

import (
	"fmt"

	"github.com/seeruk/tego/tegopb"
)

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (p *Planner) Plan() (Plan, error) {
	return Plan{}, nil
}

func (p *Planner) planEnum(enum *ProtoEnum) (EnumPlan, []Diagnostic, bool) {
	if enum.Options.GetOmit() {
		return EnumPlan{}, nil, false
	}

	underlying, diagnostics := enumUnderlyingType(enum)

	plan := EnumPlan{
		ProtoName:  enum.FullName,
		Name:       enum.GoName,
		Comment:    enum.Options.GetComment(),
		Underlying: underlying,
	}

	if enum.Options.HasName() {
		plan.Name = enum.Options.GetName()
	}

	for _, value := range enum.Values {
		constant, constantDiagnostics, ok := p.planEnumConstant(value, underlying)
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

func (p *Planner) planEnumConstant(value *ProtoEnumValue, underlying EnumUnderlyingType) (EnumConstantPlan, []Diagnostic, bool) {
	if value.Options.GetOmit() {
		return EnumConstantPlan{}, nil, false
	}

	plan := EnumConstantPlan{
		ProtoName: value.FullName,
		Name:      value.GoName,
		Comment:   value.Options.GetComment(),
	}

	if value.Options.HasName() {
		plan.Name = value.Options.GetName()
	}

	diagnostics := enumConstantValue(value, underlying, plan.Name, &plan.Value)

	return plan, diagnostics, true
}

func enumConstantValue(value *ProtoEnumValue, underlying EnumUnderlyingType, plannedName string, out *EnumConstantValue) []Diagnostic {
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
			out.String = plannedName
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

func fatalDiagnostic(path, format string, args ...any) Diagnostic {
	return Diagnostic{
		Level:   DiagnosticLevelFatal,
		Path:    path,
		Message: fmt.Sprintf(format, args...),
	}
}
