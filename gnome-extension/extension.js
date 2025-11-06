'use strict';

import St from 'gi://St';
import GLib from 'gi://GLib';
import Gio from 'gi://Gio';
import Meta from 'gi://Meta';
import Shell from 'gi://Shell';
import Clutter from 'gi://Clutter';

import { Extension } from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as PanelMenu from 'resource:///org/gnome/shell/ui/panelMenu.js';

// Visualization update interval (ms) for level bars shifting
const LEVEL_UPDATE_INTERVAL_MS = 40; // Should be synchronized with @recorder.go throttle

const REALTIME_SHORTCUT_KEY = '<Ctrl><Super>v';
const POST_AUTOPASTE_SHORTCUT_KEY = '<Ctrl><Super>c';
const POST_ROUTER_SHORTCUT_KEY = '<Ctrl><Super>d';

// Controlls visualization size
const BAR_WIDTH = 3;
const BAR_SPACING = 2;
const NUM_BARS = 20;
const CONTAINER_HEIGHT = 30;
const MAX_SCALE = 1.0;
const MIN_SCALE = 0;

const OVERLAY_PADDING_BOTTOM = 3;
const OVERLAY_PADDING_TOP = 0;
const OVERLAY_PADDING_RIGHT = 3;
const OVERLAY_PADDING_LEFT = 0;
const OVERLAY_BORDER = 2;

const CONTAINER_PADDING_TOP = 8;
const CONTAINER_PADDING_RIGHT = 0;
const CONTAINER_PADDING_BOTTOM = 0;
const CONTAINER_PADDING_LEFT = 8;

// Calculated automatically
const CONTAINER_WIDTH = 131;
const BAR_HEIGHT = (CONTAINER_HEIGHT / 2) + 1;

// Recording states
const State = {
    IDLE: 'idle',
    RECORDING: 'recording',
    UPLOADING: 'uploading',
    FINISHED: 'finished',
    CANCELED: 'canceled'
};

// D-Bus interface definition
const VoicifyDBusInterface = `
<node>
  <interface name="com.dooshek.voicify.Recorder">
    <method name="StartRealtimeRecording"/>
    <method name="TogglePostTranscriptionAutoPaste"/>
    <method name="TogglePostTranscriptionRouter"/>
    <method name="GetStatus">
      <arg name="is_recording" type="b" direction="out"/>
    </method>
    <method name="CancelRecording"/>
    <method name="UpdateFocusedWindow">
      <arg name="title" type="s" direction="in"/>
      <arg name="app" type="s" direction="in"/>
    </method>
    <signal name="RecordingStarted"/>
    <signal name="TranscriptionReady">
      <arg name="text" type="s"/>
    </signal>
    <signal name="PartialTranscription">
      <arg name="text" type="s"/>
    </signal>
    <signal name="CompleteTranscription">
      <arg name="text" type="s"/>
    </signal>
    <signal name="RecordingError">
      <arg name="error" type="s"/>
    </signal>
    <signal name="RecordingCancelled"/>
    <signal name="InputLevel">
      <arg name="level" type="d"/>
    </signal>
    <signal name="RequestPaste">
      <arg name="text" type="s"/>
    </signal>
  </interface>
</node>`;

const VoicifyProxy = Gio.DBusProxy.makeProxyWrapper(VoicifyDBusInterface);

export default class VoicifyExtension extends Extension {
    constructor(metadata) {
        super(metadata);
        this._indicator = null;
        this._icon = null;
        this._realtimeAction = null;
        this._postAutoPasteAction = null;
        this._postRouterAction = null;
        this._state = State.IDLE;
        this._timeoutId = null;
        this._waveWidget = null;
        this._waveBars = null;
        this._barTimers = null;
        this._uploadTimer = null;
        this._finishedTimer = null;
        this._dbusProxy = null;
        this._levels = [];
        this._levelTimer = null;
        this._lastShortcutTime = 0;
        this._isRealtimeMode = false; // Track current recording mode
        this._isPostAutoPaste = false; // Post-transcription auto-paste mode
        this._isPostRouter = false; // Post-transcription router mode
        this._accumulatedText = '';  // Store accumulated partial transcription
        this._debounceMs = 500; // Prevent multiple calls within 500ms
        this._virtualKeyboard = null; // Virtual keyboard device for X11 paste
    }

    enable() {
        console.debug('Voicify extension enabled');

        // Initialize virtual keyboard for X11 paste
        try {
            const seat = Clutter.get_default_backend().get_default_seat();
            this._virtualKeyboard = seat.create_virtual_device(Clutter.InputDeviceType.KEYBOARD_DEVICE);
        } catch (error) {
            console.debug('Failed to create virtual keyboard device:', error.message);
            this._virtualKeyboard = null;
        }

        // Initialize D-Bus proxy
        this._initDBusProxy();

        // Create panel indicator
        this._createIndicator();

        // Set up global shortcuts
        this._setupRealtimeShortcut();
        this._setupPostAutoPasteShortcut();
        this._setupPostRouterShortcut();
    }

    disable() {
        console.debug('Voicify extension disabled');

        // Clean up timers
        if (this._timeoutId) {
            GLib.Source.remove(this._timeoutId);
            this._timeoutId = null;
        }

        if (this._barTimers) {
            this._barTimers.forEach(timer => GLib.Source.remove(timer));
            this._barTimers = null;
        }

        if (this._uploadTimer) {
            GLib.Source.remove(this._uploadTimer);
            this._uploadTimer = null;
        }

        if (this._finishedTimer) {
            GLib.Source.remove(this._finishedTimer);
            this._finishedTimer = null;
        }

        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }

        // Clean up global shortcuts
        if (this._realtimeAction !== null) {
            global.display.ungrab_accelerator(this._realtimeAction);
            Main.wm.allowKeybinding(
                Meta.external_binding_name_for_action(this._realtimeAction),
                Shell.ActionMode.NONE
            );
            this._realtimeAction = null;
        }

        if (this._postAutoPasteAction !== null) {
            global.display.ungrab_accelerator(this._postAutoPasteAction);
            Main.wm.allowKeybinding(
                Meta.external_binding_name_for_action(this._postAutoPasteAction),
                Shell.ActionMode.NONE
            );
            this._postAutoPasteAction = null;
        }

        if (this._postRouterAction !== null) {
            global.display.ungrab_accelerator(this._postRouterAction);
            Main.wm.allowKeybinding(
                Meta.external_binding_name_for_action(this._postRouterAction),
                Shell.ActionMode.NONE
            );
            this._postRouterAction = null;
        }

        // Clean up wave widget
        if (this._waveWidget) {
            this._waveWidget.destroy();
            this._waveWidget = null;
            this._waveBars = null;
        }

        // Clean up indicator
        // Clean up D-Bus proxy
        if (this._dbusProxy) {
            this._dbusProxy = null;
        }

        if (this._indicator) {
            this._indicator.destroy();
            this._indicator = null;
        }
        this._icon = null;
    }

    _createIndicator() {
        this._indicator = new PanelMenu.Button(0.0, this.metadata.name, false);

        // Create icon and store reference
        this._icon = new St.Icon({
            icon_name: 'audio-input-microphone-symbolic',
            style_class: 'system-status-icon',
        });

        this._indicator.add_child(this._icon);
        this._indicator.connect('button-press-event', () => this._onRealtimeShortcutPressed());

        // Add to panel
        Main.panel.addToStatusArea(this.uuid, this._indicator);
    }

    _setupRealtimeShortcut() {
        this._realtimeAction = global.display.grab_accelerator(REALTIME_SHORTCUT_KEY, Meta.KeyBindingFlags.NONE);

        if (this._realtimeAction == Meta.KeyBindingAction.NONE) {
            console.error('Unable to grab accelerator for Realtime');
            return;
        }

        const name = Meta.external_binding_name_for_action(this._realtimeAction);
        Main.wm.allowKeybinding(name, Shell.ActionMode.ALL);

        global.display.connect('accelerator-activated', (display, action, deviceId, timestamp) => {
            if (action === this._realtimeAction) {
                this._onRealtimeShortcutPressed();
            }
        });

        console.debug('Realtime shortcut registered:', REALTIME_SHORTCUT_KEY);
    }

    _setupPostAutoPasteShortcut() {
        this._postAutoPasteAction = global.display.grab_accelerator(POST_AUTOPASTE_SHORTCUT_KEY, Meta.KeyBindingFlags.NONE);

        if (this._postAutoPasteAction == Meta.KeyBindingAction.NONE) {
            console.error('Unable to grab accelerator for Post Auto-Paste');
            return;
        }

        const name = Meta.external_binding_name_for_action(this._postAutoPasteAction);
        Main.wm.allowKeybinding(name, Shell.ActionMode.ALL);

        global.display.connect('accelerator-activated', (display, action, deviceId, timestamp) => {
            if (action === this._postAutoPasteAction) {
                this._onPostAutoPasteShortcutPressed();
            }
        });

        console.debug('Post auto-paste shortcut registered:', POST_AUTOPASTE_SHORTCUT_KEY);
    }

    _setupPostRouterShortcut() {
        this._postRouterAction = global.display.grab_accelerator(POST_ROUTER_SHORTCUT_KEY, Meta.KeyBindingFlags.NONE);

        if (this._postRouterAction == Meta.KeyBindingAction.NONE) {
            console.error('Unable to grab accelerator for Post Router');
            return;
        }

        const name = Meta.external_binding_name_for_action(this._postRouterAction);
        Main.wm.allowKeybinding(name, Shell.ActionMode.ALL);

        global.display.connect('accelerator-activated', (display, action, deviceId, timestamp) => {
            if (action === this._postRouterAction) {
                this._onPostRouterShortcutPressed();
            }
        });

        console.debug('Post router shortcut registered:', POST_ROUTER_SHORTCUT_KEY);
    }

    _onRealtimeShortcutPressed() {
        const currentTime = Date.now();

        if (currentTime - this._lastShortcutTime < this._debounceMs) {
            console.debug('🔥 REALTIME DEBOUNCED - ignoring rapid call');
            return;
        }
        this._lastShortcutTime = currentTime;

        console.log('🔥 REALTIME SHORTCUT PRESSED! Current state:', this._state);

        if (this._state === State.IDLE) {
            this._startRealtimeRecording();
        } else if (this._state === State.RECORDING && this._isRealtimeMode) {
            this._stopRealtimeRecording();
        } else {
            console.debug('Wrong state or mode - ignoring');
        }
    }

    _onPostAutoPasteShortcutPressed() {
        const currentTime = Date.now();

        if (currentTime - this._lastShortcutTime < this._debounceMs) {
            console.debug('🔥 POST AUTO-PASTE DEBOUNCED - ignoring rapid call');
            return;
        }
        this._lastShortcutTime = currentTime;

        console.log('🔥 POST AUTO-PASTE SHORTCUT PRESSED! Current state:', this._state);

        if (this._state === State.IDLE) {
            this._startPostAutoPasteRecording();
        } else if (this._state === State.RECORDING && this._isPostAutoPaste) {
            this._stopPostAutoPasteRecording();
        } else {
            console.debug('Wrong state or mode - ignoring');
        }
    }

    _onPostRouterShortcutPressed() {
        const currentTime = Date.now();

        if (currentTime - this._lastShortcutTime < this._debounceMs) {
            console.debug('🔥 POST ROUTER DEBOUNCED - ignoring rapid call');
            return;
        }
        this._lastShortcutTime = currentTime;

        console.log('🔥 POST ROUTER SHORTCUT PRESSED! Current state:', this._state);

        if (this._state === State.IDLE) {
            this._startPostRouterRecording();
        } else if (this._state === State.RECORDING && this._isPostRouter) {
            this._stopPostRouterRecording();
        } else {
            console.debug('Wrong state or mode - ignoring');
        }
    }


    _startRealtimeRecording() {
        console.log('🔥 _startRealtimeRecording() called');

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();

        this._isRealtimeMode = true;
        this._isPostAutoPaste = false;
        this._isPostRouter = false;
        this._accumulatedText = '';

        this._dbusProxy.StartRealtimeRecordingAsync()
            .then(() => {
                console.debug('D-Bus: StartRealtimeRecording called');
            })
            .catch(error => {
                console.error('D-Bus: Failed to call StartRealtimeRecording:', error);
                this._state = State.IDLE;
                this._isRealtimeMode = false;
                this._updateIndicator();
            });
    }

    _startPostAutoPasteRecording() {
        console.log('🔥 _startPostAutoPasteRecording() called');

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();

        this._isRealtimeMode = false;
        this._isPostAutoPaste = true;
        this._isPostRouter = false;

        this._dbusProxy.TogglePostTranscriptionAutoPasteAsync()
            .then(() => {
                console.debug('D-Bus: TogglePostTranscriptionAutoPaste called');
            })
            .catch(error => {
                console.error('D-Bus: Failed to call TogglePostTranscriptionAutoPaste:', error);
                this._state = State.IDLE;
                this._isPostAutoPaste = false;
                this._updateIndicator();
            });
    }

    _startPostRouterRecording() {
        console.log('🔥 _startPostRouterRecording() called');

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();

        this._isRealtimeMode = false;
        this._isPostAutoPaste = false;
        this._isPostRouter = true;

        this._dbusProxy.TogglePostTranscriptionRouterAsync()
            .then(() => {
                console.debug('D-Bus: TogglePostTranscriptionRouter called');
            })
            .catch(error => {
                console.error('D-Bus: Failed to call TogglePostTranscriptionRouter:', error);
                this._state = State.IDLE;
                this._isPostRouter = false;
                this._updateIndicator();
            });
    }

    _stopRealtimeRecording() {
        console.log('🔥 _stopRealtimeRecording() called');

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._dbusProxy.CancelRecordingAsync()
            .then(() => {
                console.debug('D-Bus: CancelRecording called');
                this._state = State.IDLE;
                this._isRealtimeMode = false;
                this._updateIndicator();
                this._hideWaveWidget();
            })
            .catch(error => {
                console.error('D-Bus: Failed to call CancelRecording:', error);
                this._state = State.IDLE;
                this._isRealtimeMode = false;
                this._updateIndicator();
                this._hideWaveWidget();
            });
    }

    _stopPostAutoPasteRecording() {
        console.log('🔥 _stopPostAutoPasteRecording() called');

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();

        this._state = State.UPLOADING;
        this._updateIndicator();
        this._updateWaveWidget();

        this._dbusProxy.TogglePostTranscriptionAutoPasteAsync()
            .then(() => {
                console.debug('D-Bus: TogglePostTranscriptionAutoPaste called (stop)');
            })
            .catch(error => {
                console.error('D-Bus: Failed to call TogglePostTranscriptionAutoPaste:', error);
                this._state = State.IDLE;
                this._isPostAutoPaste = false;
                this._updateIndicator();
                this._hideWaveWidget();
            });
    }

    _stopPostRouterRecording() {
        console.log('🔥 _stopPostRouterRecording() called');

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();

        this._state = State.UPLOADING;
        this._updateIndicator();
        this._updateWaveWidget();

        this._dbusProxy.TogglePostTranscriptionRouterAsync()
            .then(() => {
                console.debug('D-Bus: TogglePostTranscriptionRouter called (stop)');
            })
            .catch(error => {
                console.error('D-Bus: Failed to call TogglePostTranscriptionRouter:', error);
                this._state = State.IDLE;
                this._isPostRouter = false;
                this._updateIndicator();
                this._hideWaveWidget();
            });
    }

    _onTranscriptionReady(text) {
        console.debug('Transcription ready (daemon final):', text);

        if (this._isRealtimeMode) {
            // In realtime mode, we don't change UI; we already hid on cancel
            return;
        }

        if (this._isPostRouter) {
            // Post-transcription router mode: backend routed through router, no auto-paste
            // Plugin will emit RequestPaste signal if it wants to paste
            console.debug('Post-router mode - waiting for plugin RequestPaste signal');
            this._state = State.FINISHED;
            this._isPostRouter = false;
            this._updateIndicator();
            this._startFinishedAnimation();
        } else if (this._isPostAutoPaste) {
            // Post-transcription auto-paste mode: backend copied to clipboard, paste and show animation
            console.debug('Post-autopaste mode - performing auto-paste');
            this._performAutoPaste();
            this._state = State.FINISHED;
            this._isPostAutoPaste = false;
            this._updateIndicator();
            this._startFinishedAnimation();
        }
    }

    _onCompleteTranscription(text) {
        console.debug('Complete transcription received:', text);

        // Paste only on complete chunks in realtime mode
        if (this._isRealtimeMode && text && text.length > 0) {
            this._injectTextDelta(text + ' ');
        }
    }

    _onRequestPaste(text) {
        console.debug('RequestPaste signal received from plugin:', text);

        // Copy to clipboard
        St.Clipboard.get_default().set_text(St.ClipboardType.CLIPBOARD, text);

        // Trigger paste with small delay
        GLib.timeout_add(GLib.PRIORITY_DEFAULT, 100, () => {
            this._performAutoPaste();
            return GLib.SOURCE_REMOVE;
        });
    }

    _injectTextDelta(text) {
        console.debug('Injecting text delta:', text);

        // Copy to clipboard and trigger paste
        St.Clipboard.get_default().set_text(St.ClipboardType.CLIPBOARD, text);

        // Use a small delay to ensure clipboard is set before paste
        GLib.timeout_add(GLib.PRIORITY_DEFAULT, 100, () => {
            this._performAutoPaste();
            return GLib.SOURCE_REMOVE;
        });
    }

    _onRecordingCancelled() {
        // Realtime: simply return to idle and hide the widget, no animation
        this._state = State.IDLE;
        this._isRealtimeMode = false;
        this._isPostAutoPaste = false;
        this._isPostRouter = false;
        this._updateIndicator();
        this._hideWaveWidget();
        console.debug('Recording cancelled');
    }

    _performAutoPaste() {
        try {
            if (!this._virtualKeyboard) {
                console.debug('Virtual keyboard not available');
                return;
            }

            const eventTime = global.get_current_time();

            // Send Ctrl+V
            this._virtualKeyboard.notify_keyval(eventTime, Clutter.KEY_Control_L, Clutter.KeyState.PRESSED);
            this._virtualKeyboard.notify_keyval(eventTime + 10, Clutter.KEY_v, Clutter.KeyState.PRESSED);
            this._virtualKeyboard.notify_keyval(eventTime + 20, Clutter.KEY_v, Clutter.KeyState.RELEASED);
            this._virtualKeyboard.notify_keyval(eventTime + 30, Clutter.KEY_Control_L, Clutter.KeyState.RELEASED);

            console.debug('Auto-paste performed with virtual keyboard');
        } catch (error) {
            console.debug('Virtual keyboard failed (likely Wayland):', error.message);
            console.debug('Text available in clipboard for manual paste');
        }
    }

    _updateIndicator() {
        if (!this._icon) return;

        switch (this._state) {
            case State.IDLE:
                this._icon.icon_name = 'audio-input-microphone-symbolic';
                this._icon.style_class = 'system-status-icon';
                break;

            case State.RECORDING:
                this._icon.icon_name = 'media-record-symbolic';
                this._icon.style_class = 'system-status-icon recording';
                break;

            case State.UPLOADING:
                this._icon.icon_name = 'folder-upload-symbolic';
                this._icon.style_class = 'system-status-icon uploading';
                break;

            case State.FINISHED:
                this._icon.icon_name = 'emblem-ok-symbolic';
                this._icon.style_class = 'system-status-icon finished';
                break;

            case State.CANCELED:
                this._icon.icon_name = 'process-stop-symbolic';
                this._icon.style_class = 'system-status-icon canceled';
                break;
        }
    }

    _showWaveWidget() {
        console.log('🔥 _showWaveWidget() called');

        if (this._waveWidget) {
            console.log('🔥 Widget already exists, returning');
            return;
        }

        console.log('🔥 Creating wave widget...');

        // Create wave overlay widget
        this._waveWidget = new St.Widget({
            style_class: 'voicify-wave-overlay',
            style: 'padding: ' + OVERLAY_PADDING_TOP + 'px ' + OVERLAY_PADDING_RIGHT + 'px ' + OVERLAY_PADDING_BOTTOM + 'px ' + OVERLAY_PADDING_LEFT + 'px; border-width: ' + OVERLAY_BORDER + 'px;',
            reactive: true,
            can_focus: false,
            track_hover: false,
            visible: true,
        });

        // Create wave container
        const waveContainer = new St.BoxLayout({
            style_class: 'voicify-wave-container',
            style: 'padding:' + CONTAINER_PADDING_TOP + 'px ' + CONTAINER_PADDING_RIGHT + 'px ' + CONTAINER_PADDING_BOTTOM + 'px ' + CONTAINER_PADDING_LEFT + 'px;',
            vertical: false,
            x_align: Clutter.ActorAlign.CENTER,
            y_align: Clutter.ActorAlign.CENTER,
            clip_to_allocation: true,
        });

        // Create equalizer bars
        this._waveBars = [];
        for (let i = 0; i < NUM_BARS - 1; i++) {
            const bar = new St.Widget({
                style_class: `voicify-wave-bar`,
                style: `width: ${BAR_WIDTH}px; margin-right: ${BAR_SPACING}px; height: ${BAR_HEIGHT}px;`,
                visible: true,
                clip_to_allocation: true,
            });
            this._waveBars.push(bar);
            waveContainer.add_child(bar);
            console.log(`🔥 Created bar ${i} with height: ${bar.height}`);
        }

        waveContainer.set_size(CONTAINER_WIDTH, CONTAINER_HEIGHT);

        this._waveWidget.add_child(waveContainer);

        // Add as chrome (ensures visibility over UI)
        Main.layoutManager.addChrome(this._waveWidget);

        // Position at bottom left of center
        const monitor = Main.layoutManager.primaryMonitor;
        this._waveWidget.set_position(
            monitor.x + (monitor.width - CONTAINER_WIDTH) / 2,
            monitor.y + monitor.height * 0.98 - 20
        );

        console.debug(`Wave widget positioned at: ${monitor.x + monitor.width / 2 - 60}, ${monitor.y + monitor.height * 0.98 - 12}`);

        // Initialize bars and start level-driven rendering
        this._initFlatBars();
    }

    _updateWaveWidget() {
        if (!this._waveWidget) return;

        // Change to upload animation
        this._waveWidget.style_class = 'voicify-wave-overlay uploading';
        this._stopLevelWave();
        this._startUploadAnimation();
    }

    _hideWaveWidget() {
        // Clean up all animations
        if (this._barTimers) {
            this._barTimers.forEach(timer => GLib.Source.remove(timer));
            this._barTimers = null;
        }

        if (this._uploadTimer) {
            GLib.Source.remove(this._uploadTimer);
            this._uploadTimer = null;
        }

        if (this._finishedTimer) {
            GLib.Source.remove(this._finishedTimer);
            this._finishedTimer = null;
        }

        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }

        if (this._waveWidget) {
            this._waveWidget.destroy();
            this._waveWidget = null;
            this._waveBars = null;
        }
        console.debug('Wave widget hidden');
    }

    _startFinishedAnimation() {
        // Stop upload animation
        if (this._uploadTimer) {
            GLib.Source.remove(this._uploadTimer);
            this._uploadTimer = null;
        }
        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }

        if (!this._waveWidget || !this._waveBars) return;

        // Change widget class to finished
        this._waveWidget.style_class = 'voicify-wave-overlay finished';

        console.debug('Starting finished animation - start at 100% then shrink to 0');

        // Set all bars to 100% immediately
        this._waveBars.forEach(bar => {
            bar.scale_y = MAX_SCALE; // Start at max scale
        });

        // Short pause then animate down to 0
        this._finishedTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 150, () => {
            this._finishedTimer = null;

            let phase = 0;
            this._finishedTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 30, () => {
                // Animate all bars down to 0 simultaneously
                const progress = phase / 25;
                const scale = MAX_SCALE * (1 - progress); // Shrink from max to 0

                this._waveBars.forEach(bar => {
                    bar.scale_y = Math.max(0.05, scale);
                });

                phase++;

                if (phase >= 25) {
                    // Animation complete - hide widget and reset
                    this._finishedTimer = null;
                    this._state = State.IDLE;
                    this._updateIndicator();
                    this._hideWaveWidget();
                    console.debug('Finished animation complete - back to idle');
                    return GLib.SOURCE_REMOVE;
                }

                return GLib.SOURCE_CONTINUE;
            });

            return GLib.SOURCE_REMOVE;
        });
    }

    _startCanceledAnimation() {
        // Stop upload animation
        if (this._uploadTimer) {
            GLib.Source.remove(this._uploadTimer);
            this._uploadTimer = null;
        }
        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }

        if (!this._waveWidget || !this._waveBars) return;

        // Change widget class to canceled
        this._waveWidget.style_class = 'voicify-wave-overlay canceled';

        console.debug('Starting canceled animation - start at 100% then shrink to 0');

        // Set all bars to 100% immediately
        this._waveBars.forEach(bar => {
            bar.scale_y = MAX_SCALE; // Start at max scale
        });

        // Short pause then animate down to 0
        this._finishedTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, ANIMATION_DURATION, () => {
            this._finishedTimer = null;

            let phase = 0;
            this._finishedTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 30, () => {
                // Animate all bars down to 0 simultaneously
                const progress = phase / 25;
                const scale = MAX_SCALE * (1 - progress); // Shrink from max to 0

                this._waveBars.forEach(bar => {
                    bar.scale_y = Math.max(MIN_SCALE, scale);
                });

                phase++;

                if (phase >= 25) {
                    // Animation complete - hide widget and reset
                    this._finishedTimer = null;
                    this._state = State.IDLE;
                    this._updateIndicator();
                    this._hideWaveWidget();
                    console.debug('Canceled animation complete - back to idle');
                    return GLib.SOURCE_REMOVE;
                }

                return GLib.SOURCE_CONTINUE;
            });

            return GLib.SOURCE_REMOVE;
        });
    }

    _startUploadAnimation() {
        // Stop recording animations first
        if (this._barTimers) {
            this._barTimers.forEach(timer => GLib.Source.remove(timer));
            this._barTimers = [];
        }
        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }

        if (!this._waveBars) return;

        console.debug('Starting upload wave animation - sinusoidal wave left to right');

        let waveOffset = 0;
        const waveSpeed = 0.3;

        this._uploadTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 60, () => {
            if (this._state !== State.UPLOADING) {
                return GLib.SOURCE_REMOVE;
            }

            this._waveBars.forEach((bar, index) => {
                // Create sinusoidal wave moving RIGHT to LEFT
                const phase = (index * 0.8) - waveOffset; // Reverse direction with minus
                const amplitude = 0.4 + Math.sin(phase) * 0.3; // Wave amplitude
                const scale = 0.3 + Math.abs(amplitude);

                bar.scale_y = Math.max(MIN_SCALE, Math.min(MAX_SCALE, scale));
            });

            waveOffset += waveSpeed;
            return GLib.SOURCE_CONTINUE;
        });
    }

    _initDBusProxy() {
        console.debug('Initializing D-Bus proxy for voicify daemon');

        this._dbusProxy = new VoicifyProxy(
            Gio.DBus.session,
            'com.dooshek.voicify',
            '/com/dooshek/voicify/Recorder'
        );

        // Connect to D-Bus signals
        this._dbusProxy.connectSignal('RecordingStarted', () => {
            console.debug('🔥 D-Bus: Recording started signal received, changing state to RECORDING');
            this._state = State.RECORDING;
            this._updateIndicator();
            this._showWaveWidget();
            this._startLevelWave();
        });

        this._dbusProxy.connectSignal('TranscriptionReady', (proxy, sender, [text]) => {
            console.debug('D-Bus: Transcription ready signal received:', text);
            this._onTranscriptionReady(text);
        });

        this._dbusProxy.connectSignal('PartialTranscription', (proxy, sender, [text]) => {
            console.debug('D-Bus: Partial transcription signal received:', text);
            this._onPartialTranscription(text);
        });

        this._dbusProxy.connectSignal('CompleteTranscription', (proxy, sender, [text]) => {
            console.debug('D-Bus: Complete transcription signal received:', text);
            this._onCompleteTranscription(text);
        });

        this._dbusProxy.connectSignal('RecordingError', (proxy, sender, [error]) => {
            console.error('D-Bus: Recording error signal received:', error);
            this._state = State.IDLE;
            this._updateIndicator();
            this._stopLevelWave();
            this._hideWaveWidget();
        });

        this._dbusProxy.connectSignal('RecordingCancelled', () => {
            console.debug('D-Bus: Recording cancelled signal received');
            this._onRecordingCancelled();
        });

        // Live input level
        this._dbusProxy.connectSignal('InputLevel', (proxy, sender, [level]) => {
            console.debug('🔥 D-Bus: InputLevel signal received:', level, 'state:', this._state);
            if (this._state !== State.RECORDING) {
                console.debug('🔥 InputLevel ignored - not in RECORDING state');
                return;
            }
            if (typeof level === 'number' && isFinite(level)) {
                console.debug('🔥 Pushing level to visualization:', level);
                this._pushLevel(level);
            } else {
                console.debug('🔥 Invalid level value:', level, 'type:', typeof level);
            }
        });

        // Request paste from plugins
        this._dbusProxy.connectSignal('RequestPaste', (proxy, sender, [text]) => {
            console.debug('D-Bus: RequestPaste signal received from plugin');
            this._onRequestPaste(text);
        });

        console.debug('🔥 D-Bus proxy initialized with signals connected');
    }

    _initFlatBars() {
        if (!this._waveBars) return;
        this._levels = [];
        for (let i = 0; i < this._waveBars.length; i++) this._levels.push(0);
        this._waveBars.forEach(bar => {
            bar.set_pivot_point(0.0, 1.0);
            bar.scale_y = MIN_SCALE;
        });
    }

    _pushLevel(level) {
        if (!this._waveBars) return;
        // EMA smoothing to reduce jitter
        const raw = Math.max(MIN_SCALE, Math.min(MAX_SCALE, level));
        const alpha = 0.25; // smoothing factor (higher = more responsive)
        const last = this._levels.length > 0 ? this._levels[this._levels.length - 1] : raw;
        const smoothed = alpha * raw + (1 - alpha) * last;
        this._levels.push(smoothed);
        if (this._levels.length > this._waveBars.length) {
            this._levels.shift();
        }
    }

    _startLevelWave() {
        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }
        this._initFlatBars();
        this._levelTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, LEVEL_UPDATE_INTERVAL_MS, () => {
            if (this._state !== State.RECORDING || !this._waveBars) {
                return GLib.SOURCE_REMOVE;
            }
            const n = this._waveBars.length;
            for (let i = 0; i < n; i++) {
                const levelIdx = this._levels.length - 1 - i;
                const val = levelIdx >= 0 ? this._levels[levelIdx] : 0.05;
                this._waveBars[n - 1 - i].scale_y = Math.max(MIN_SCALE, Math.min(MAX_SCALE, val));
            }
            return GLib.SOURCE_CONTINUE;
        });
    }

    _stopLevelWave() {
        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }
        this._levels = [];
    }

    _getFocusedWindow() {
        try {
            // Get focused window using Shell's window tracker
            const windowTracker = Shell.WindowTracker.get_default();
            const focusedWindow = global.display.get_focus_window();

            if (!focusedWindow) {
                return { title: '', app: '' };
            }

            const title = focusedWindow.get_title() || '';
            const app = focusedWindow.get_wm_class() || '';

            console.debug(`Focused window - title: "${title}", app: "${app}"`);
            return { title, app };
        } catch (error) {
            console.error('Error getting focused window:', error);
            return { title: '', app: '' };
        }
    }

    _updateFocusedWindowInDaemon() {
        const { title, app } = this._getFocusedWindow();

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._dbusProxy.UpdateFocusedWindowAsync(title, app)
            .then(() => {
                console.debug('D-Bus: UpdateFocusedWindow called successfully');
            })
            .catch(error => {
                console.error('D-Bus: Failed to call UpdateFocusedWindow:', error);
            });
    }
}
