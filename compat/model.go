package compat

type Field struct {
	FQN        string `json:"fqn"`
	Deprecated bool   `json:"deprecated,omitempty"`
}

type ManagedObject struct {
	Field
	Methods    map[string]*Field `json:"methods,omitempty"`
	Properties map[string]*Field `json:"properties,omitempty"`
}

type DataObject struct {
	Field
	Properties map[string]*Field `json:"properties,omitempty"`
}

type Enum struct {
	Field
	Constants map[string]*Field `json:"constants,omitempty"`
}

type Fault struct {
	Field
	Properties map[string]*Field `json:"properties,omitempty"`
}

type API struct {
	Version        string                    `json:"version"`
	ManagedObjects map[string]*ManagedObject `json:"managedObjects,omitempty"`
	DataObjects    map[string]*DataObject    `json:"dataObjects,omitempty"`
	Enums          map[string]*Enum          `json:"enums,omitempty"`
	Faults         map[string]*Fault         `json:"faults,omitempty"`
}

func (a *API) GetDeprecatedMethods() map[string]*Field {
	methods := make(map[string]*Field)
	for _, o := range a.ManagedObjects {
		for n, m := range o.Methods {
			if m.Deprecated {
				methods[n] = m
			}
		}
	}
	return methods
}

func (a *API) GetDeprecatedProperties() map[string]*Field {
	fields := make(map[string]*Field)
	for _, o := range a.ManagedObjects {
		for n, m := range o.Properties {
			if m.Deprecated {
				fields[n] = m
			}
		}
	}
	for _, o := range a.DataObjects {
		for n, m := range o.Properties {
			if m.Deprecated {
				fields[n] = m
			}
		}
	}
	for _, o := range a.Enums {
		for n, m := range o.Constants {
			if m.Deprecated {
				fields[n] = m
			}
		}
	}
	for _, o := range a.Faults {
		for n, m := range o.Properties {
			if m.Deprecated {
				fields[n] = m
			}
		}
	}
	return fields
}
