package mydbus

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/electricface/dbus"
)

func newIntrospectIntf(h *Handler) *exportedIntf {
	methods := make(map[string]dbus.Method)
	methods["Introspect"] = exportedMethod{
		value: reflect.ValueOf(func(msg dbus.Message) (string, *dbus.Error) {
			path := msg.Headers[dbus.FieldPath].Value().(dbus.ObjectPath)
			return h.introspectPath(path), nil
		}),
		fn: func(args []interface{}) ([]interface{}, error) {
			// TODO
			return nil, nil
		},
	}
	return newExportedIntf(methods, true)
}

//NewDefaultHandler returns an instance of the default
//call handler. This is useful if you want to implement only
//one of the two handlers but not both.
func newHandler() *Handler {
	h := &Handler{
		objects:     make(map[dbus.ObjectPath]*exportedObj),
		defaultIntf: make(map[string]*exportedIntf),
	}
	h.defaultIntf["org.freedesktop.DBus.Introspectable"] = newIntrospectIntf(h)
	return h
}

type Handler struct {
	sync.RWMutex
	objects     map[dbus.ObjectPath]*exportedObj
	defaultIntf map[string]*exportedIntf
}

func (h *Handler) PathExists(path dbus.ObjectPath) bool {
	_, ok := h.objects[path]
	return ok
}

func (h *Handler) introspectPath(path dbus.ObjectPath) string {
	subpath := make(map[string]struct{})
	var xml bytes.Buffer
	xml.WriteString("<node>")
	for obj := range h.objects {
		p := string(path)
		if p != "/" {
			p += "/"
		}
		if strings.HasPrefix(string(obj), p) {
			node_name := strings.Split(string(obj[len(p):]), "/")[0]
			subpath[node_name] = struct{}{}
		}
	}
	for s := range subpath {
		xml.WriteString("\n\t<node name=\"" + s + "\"/>")
	}
	xml.WriteString("\n</node>")
	return xml.String()
}

func (h *Handler) LookupObject(path dbus.ObjectPath) (dbus.ServerObject, bool) {
	h.RLock()
	defer h.RUnlock()
	object, ok := h.objects[path]
	if ok {
		return object, ok
	}

	// If an object wasn't found for this exact path,
	// look for a matching subtree registration
	subtreeObject := newExportedObject()
	path = path[:strings.LastIndex(string(path), "/")]
	for len(path) > 0 {
		object, ok = h.objects[path]
		if ok {
			for name, iface := range object.interfaces {
				// Only include this handler if it registered for the subtree
				if iface.isFallbackInterface() {
					subtreeObject.interfaces[name] = iface
				}
			}
			break
		}

		path = path[:strings.LastIndex(string(path), "/")]
	}

	for name, intf := range h.defaultIntf {
		if _, exists := subtreeObject.interfaces[name]; exists {
			continue
		}
		subtreeObject.interfaces[name] = intf
	}

	return subtreeObject, true
}

func (h *Handler) AddObject(path dbus.ObjectPath, object *exportedObj) {
	h.Lock()
	h.objects[path] = object
	h.Unlock()
}

type HandlerFunc func(args []interface{}) ([]interface{}, error)

type MethodItem struct {
	Value reflect.Value
	Fn    HandlerFunc
}

func (h *Handler) Export(methods map[string]MethodItem, path dbus.ObjectPath, iface string) error {
	return h.export(methods, path, iface)
}

func (h *Handler) export(methods map[string]MethodItem, path dbus.ObjectPath, iface string) error {
	if !path.IsValid() {
		return fmt.Errorf(`dbus: Invalid path name: "%s"`, path)
	}
	if methods == nil {
		// unexport
		// TODO
		return nil
	}
	// If this is the first handler for this path, make a new map to hold all
	// handlers for this path.
	if !h.PathExists(path) {
		h.AddObject(path, newExportedObject())
	}

	exportedMethods := make(map[string]dbus.Method)
	for name, method := range methods {
		exportedMethods[name] = exportedMethod{value: method.Value, fn: method.Fn}
	}

	// Finally, save this handler
	obj := h.objects[path]
	obj.AddInterface(iface, newExportedIntf(exportedMethods, false))

	return nil
}

func (h *Handler) DeleteObject(path dbus.ObjectPath) {
	h.Lock()
	delete(h.objects, path)
	h.Unlock()
}

type exportedMethod struct {
	value reflect.Value
	fn    HandlerFunc
}

//var errType = reflect.TypeOf((*error)(nil)).Elem()

func (m exportedMethod) Call(args ...interface{}) ([]interface{}, error) {
	newArgs := make([]interface{}, len(args))
	for i := 0; i < len(args); i++ {
		newArgs[i] = reflect.ValueOf(args[i]).Elem().Interface()
	}

	results, err := m.fn(newArgs)
	if busErr, ok := err.(*dbus.Error); ok {
		if busErr == nil {
			err = nil
		}
	}
	return results, err
	//t := m.Type()
	//
	//params := make([]reflect.Value, len(args))
	//for i := 0; i < len(args); i++ {
	//	params[i] = reflect.ValueOf(args[i]).Elem()
	//}
	//
	//ret := m.Value.Call(params)
	//var err error
	//nilErr := false // The reflection will find almost-nils, let's only pass back clean ones!
	//if t.NumOut() > 0 {
	//	if e, ok := ret[t.NumOut()-1].Interface().(*dbus.Error); ok { // godbus *Error
	//		nilErr = ret[t.NumOut()-1].IsNil()
	//		ret = ret[:t.NumOut()-1]
	//		err = e
	//	} else if ret[t.NumOut()-1].Type().Implements(errType) { // Go error
	//		i := ret[t.NumOut()-1].Interface()
	//		if i == nil {
	//			nilErr = ret[t.NumOut()-1].IsNil()
	//		} else {
	//			err = i.(error)
	//		}
	//		ret = ret[:t.NumOut()-1]
	//	}
	//}
	//out := make([]interface{}, len(ret))
	//for i, val := range ret {
	//	out[i] = val.Interface()
	//}
	//if nilErr || err == nil {
	//	//concrete type to interface nil is a special case
	//	return out, nil
	//}
	//return out, err
}

func (m exportedMethod) NumArguments() int {
	return m.value.Type().NumIn()
}

func (m exportedMethod) ArgumentValue(i int) interface{} {
	return reflect.Zero(m.value.Type().In(i)).Interface()
}

func (m exportedMethod) NumReturns() int {
	return m.value.Type().NumOut()
}

func (m exportedMethod) ReturnValue(i int) interface{} {
	return reflect.Zero(m.value.Type().Out(i)).Interface()
}

func newExportedObject() *exportedObj {
	return &exportedObj{
		interfaces: make(map[string]*exportedIntf),
	}
}

type exportedObj struct {
	mu         sync.RWMutex
	interfaces map[string]*exportedIntf
}

func (obj *exportedObj) LookupInterface(name string) (dbus.Interface, bool) {
	if name == "" {
		return obj, true
	}
	obj.mu.RLock()
	defer obj.mu.RUnlock()
	intf, exists := obj.interfaces[name]
	return intf, exists
}

func (obj *exportedObj) AddInterface(name string, iface *exportedIntf) {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	obj.interfaces[name] = iface
}

func (obj *exportedObj) DeleteInterface(name string) {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	delete(obj.interfaces, name)
}

func (obj *exportedObj) LookupMethod(name string) (dbus.Method, bool) {
	obj.mu.RLock()
	defer obj.mu.RUnlock()
	for _, intf := range obj.interfaces {
		method, exists := intf.LookupMethod(name)
		if exists {
			return method, exists
		}
	}
	return nil, false
}

func (obj *exportedObj) isFallbackInterface() bool {
	return false
}

func newExportedIntf(methods map[string]dbus.Method, includeSubtree bool) *exportedIntf {
	return &exportedIntf{
		methods:        methods,
		includeSubtree: includeSubtree,
	}
}

type exportedIntf struct {
	methods map[string]dbus.Method

	// Whether or not this export is for the entire subtree
	includeSubtree bool
}

func (obj *exportedIntf) LookupMethod(name string) (dbus.Method, bool) {
	out, exists := obj.methods[name]
	return out, exists
}

func (obj *exportedIntf) isFallbackInterface() bool {
	return obj.includeSubtree
}
