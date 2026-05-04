package dbus

// InterfaceVersion is auto-bumped by deploy.sh based on the hash of the
// VoicifyDBusInterface XML literal in gnome-extension/extension.js. The
// extension reads it via GetInterfaceVersion at enable() and warns the
// user when the two sides have drifted (e.g. SyncThing copied a newer
// extension to a host whose daemon binary is still old).
const InterfaceVersion = "d63c611f7eec"
