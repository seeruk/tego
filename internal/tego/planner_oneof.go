package tego

import "google.golang.org/protobuf/compiler/protogen"

func (p *Planner) planOneof(oneof *ProtoOneof, si *ShapeIndex) (OneofPlan, []Diagnostic) {
	name := plannedOneofName(oneof)
	plan := OneofPlan{
		ProtoName:    oneof.FullName,
		Name:         name,
		MarkerMethod: plannedOneofMarkerMethod(name),
		Comment: plannedComment(
			"",
			false,
			oneofLeadingComment(oneof),
			string(oneof.Name),
			name,
		),
	}

	var diagnostics []Diagnostic
	for _, field := range oneof.Fields {
		if field.Options.GetOmit() {
			continue
		}

		variant, variantDiagnostics := p.planOneofVariant(field, si)
		diagnostics = append(diagnostics, variantDiagnostics...)
		plan.Variants = append(plan.Variants, variant)
	}

	return plan, diagnostics
}

func (p *Planner) planOneofStructField(oneof *ProtoOneof) FieldPlan {
	name := plannedOneofFieldName(oneof)
	return FieldPlan{
		ProtoName: oneof.FullName,
		Name:      name,
		Tags:      oneofStructTags(oneof),
		Type: TypePlan{
			Kind: TypeKindOneof,
			Ref:  plannedOneofRef(oneof),
		},
		Comment: plannedComment(
			"",
			false,
			oneofLeadingComment(oneof),
			string(oneof.Name),
			name,
		),
	}
}

func (p *Planner) planOneofVariant(field *ProtoField, si *ShapeIndex) (OneofVariantPlan, []Diagnostic) {
	name := plannedOneofVariantName(field)
	typ, diagnostics := p.planOneofVariantType(field, si)
	tags, tagDiagnostics := structTags(field)
	diagnostics = append(diagnostics, tagDiagnostics...)

	return OneofVariantPlan{
		ProtoName: field.FullName,
		Name:      name,
		FieldName: plannedFieldName(field),
		Type:      typ,
		Comment: plannedComment(
			field.Options.GetComment(),
			field.Options.HasComment(),
			fieldLeadingComment(field),
			string(field.Name),
			name,
		),
		Tags: tags,
	}, diagnostics
}

func (p *Planner) planOneofVariantType(field *ProtoField, si *ShapeIndex) (TypePlan, []Diagnostic) {
	plan, diagnostics := p.planFieldBaseType(field, si)
	if field.Options.GetNullable() {
		plan = pointerType(plan)
	}
	return plan, diagnostics
}

func oneofLeadingComment(oneof *ProtoOneof) protogen.Comments {
	if oneof == nil || oneof.Desc == nil {
		return ""
	}
	return oneof.Desc.Comments.Leading
}

func plannedOneofRef(oneof *ProtoOneof) GoTypeRef {
	ref := GoTypeRef{Name: plannedOneofName(oneof)}
	if oneof != nil && oneof.File != nil && oneof.File.Options != nil && oneof.File.Options.HasGoPackage() {
		ref.ImportPath = packageRef(oneof.File.Options.GetGoPackage()).ImportPath
	}
	return ref
}
