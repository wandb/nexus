package nexuslib

import (
	"fmt"
	"reflect"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/wandb/wandb/nexus/pkg/service"
)


func reflectEncodeToDict(v reflect.Value) map[int]string {
	m := make(map[int]string)
    for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		str := v.Type().Field(i)
		fmt.Printf("got %d %s %+v\n", i, str.Name, field)
	}
	return m
}

func reflectProtoToDict(pm protoreflect.Message) map[int]string {
	m := make(map[int]string)
	fds := pm.Descriptor().Fields()
	for k := 0; k < fds.Len(); k++ {
		fd := fds.Get(k)
		fmt.Printf("got %d %+v\n", k, fd)
	}
	return m
}

func ProtoEncodeToDict(p *service.TelemetryRecord) map[int]string {
	// return reflectEncodeToDict(reflect.ValueOf(*p))
	pm := p.ProtoReflect()
	return reflectProtoToDict(pm)
}

/*
def proto_encode_to_dict(
    pb_obj: Union["tpb.TelemetryRecord", "pb.MetricRecord"]
) -> Dict[int, Any]:
    data: Dict[int, Any] = dict()
    fields = pb_obj.ListFields()
    for desc, value in fields:
        if desc.name.startswith("_"):
            continue
        if desc.type == desc.TYPE_STRING:
            data[desc.number] = value
        elif desc.type == desc.TYPE_INT32:
            data[desc.number] = value
        elif desc.type == desc.TYPE_ENUM:
            data[desc.number] = value
        elif desc.type == desc.TYPE_MESSAGE:
            nested = value.ListFields()
            bool_msg = all(d.type == d.TYPE_BOOL for d, _ in nested)
            if bool_msg:
                items = [d.number for d, v in nested if v]
                if items:
                    data[desc.number] = items
            else:
                # TODO: for now this code only handles sub-messages with strings
                md = {}
                for d, v in nested:
                    if not v or d.type != d.TYPE_STRING:
                        continue
                    md[d.number] = v
                data[desc.number] = md
    return data
*/
