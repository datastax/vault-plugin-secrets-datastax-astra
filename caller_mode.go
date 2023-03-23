package datastax_astra

import "strings"

type CallerMode int

const (
    UndefinedCallerMode CallerMode = iota
    StandardCallerMode
    SidecarCallerMode
)

func (cm CallerMode) String() string {
    switch cm {
    case StandardCallerMode:
        return "standard"
    case SidecarCallerMode:
        return "sidecar"
    }
    return "unknown"
}

func getCallerModeFromString(callerMode string) CallerMode {
    if strings.EqualFold(callerMode, "standard") {
        return StandardCallerMode
    } else if strings.EqualFold(callerMode, "sidecar") {
        return SidecarCallerMode
    }
    return UndefinedCallerMode
}