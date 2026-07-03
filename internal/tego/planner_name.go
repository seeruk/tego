package tego

import (
	"strings"

	"github.com/danielgtaylor/casing"
)

func plannedMessageName(message *ProtoMessage) string {
	if message.Options.HasName() {
		return message.Options.GetName()
	}
	if message.Parent != nil {
		return plannedMessageName(message.Parent) + goName(string(message.Name))
	}
	return goName(string(message.Name))
}

func plannedServiceName(service *ProtoService) string {
	return goName(string(service.Name))
}

func plannedServiceClientName(serviceName string) string {
	return serviceName + "Client"
}

func plannedServiceGRPCServerName(serviceName string) string {
	return tempIdentifierBase(serviceName) + "GRPCServer"
}

func plannedServiceGRPCClientName(serviceName string) string {
	return tempIdentifierBase(serviceName) + "GRPCClient"
}

func plannedServiceGRPCRegisterName(serviceName string) string {
	return "Register" + serviceName + "GRPCServer"
}

func plannedServiceGRPCNewClientName(serviceName string) string {
	return "New" + serviceName + "GRPCClient"
}

func plannedServiceConnectHandlerName(serviceName string) string {
	return tempIdentifierBase(serviceName) + "ConnectHandler"
}

func plannedServiceConnectClientName(serviceName string) string {
	return tempIdentifierBase(serviceName) + "ConnectClient"
}

func plannedServiceConnectNewHandlerName(serviceName string) string {
	return "New" + serviceName + "ConnectHandler"
}

func plannedServiceConnectNewClientName(serviceName string) string {
	return "New" + serviceName + "ConnectClient"
}

func plannedMethodName(method *ProtoMethod) string {
	return goName(string(method.Name))
}

func plannedFieldName(field *ProtoField) string {
	if field.Options.HasName() {
		return field.Options.GetName()
	}
	return goName(string(field.Name))
}

func plannedEnumName(enum *ProtoEnum) string {
	if enum.Options.HasName() {
		return enum.Options.GetName()
	}
	if enum.Parent != nil {
		return plannedMessageName(enum.Parent) + goName(string(enum.Name))
	}
	return goName(string(enum.Name))
}

func plannedEnumConstantName(value *ProtoEnumValue, enumName string) string {
	if value.Options.HasName() {
		return value.Options.GetName()
	}
	return enumName + goName(enumValueSuffix(value))
}

func plannedOneofName(oneof *ProtoOneof) string {
	return plannedMessageName(oneof.Parent) + goName(string(oneof.Name))
}

func plannedOneofFieldName(oneof *ProtoOneof) string {
	return goName(string(oneof.Name))
}

func plannedOneofVariantName(field *ProtoField) string {
	return plannedMessageName(field.Parent) + goName(string(field.Name))
}

func plannedOneofMarkerMethod(name string) string {
	if name == "" {
		return "isOneof"
	}
	return "is" + name
}

func plannedReceiverName(name string) string {
	words := casing.Split(name)
	if len(words) == 0 {
		// This should only happen for an empty planned type name, but keep the
		// generated method receiver valid while diagnostics catch the real issue.
		return "v"
	}

	var receiver strings.Builder
	for _, word := range words {
		lower := strings.ToLower(word)
		receiver.WriteByte(lower[0])
	}
	return receiver.String()
}

func enumValueSuffix(value *ProtoEnumValue) string {
	if value.Parent == nil {
		return string(value.Name)
	}

	prefix := strings.ToUpper(casing.Snake(string(value.Parent.Name))) + "_"
	suffix, ok := strings.CutPrefix(string(value.Name), prefix)
	if !ok || suffix == "" {
		return string(value.Name)
	}
	return suffix
}

func goName(name string) string {
	return casing.Camel(name, strings.ToLower, goInitialism)
}

func goInitialism(part string) string {
	initialism := casing.Initialism(part)
	if initialism != part {
		return initialism
	}

	singular, ok := strings.CutSuffix(part, "s")
	if !ok {
		return part
	}

	initialism = casing.Initialism(singular)
	if initialism == singular {
		return part
	}

	return initialism + "s"
}
