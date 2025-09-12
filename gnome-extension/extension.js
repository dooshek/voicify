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

const SHORTCUT_KEY = '<Ctrl><Super>v';

// Recording states
const State = {
    IDLE: 'idle',
    RECORDING: 'recording',
    UPLOADING: 'uploading',
    FINISHED: 'finished'
};

// D-Bus interface definition
const VoicifyDBusInterface = `
<node>
  <interface name="com.dooshek.voicify.Recorder">
    <method name="ToggleRecording"/>
    <method name="GetStatus">
      <arg name="is_recording" type="b" direction="out"/>
    </method>
    <signal name="RecordingStarted"/>
    <signal name="TranscriptionReady">
      <arg name="text" type="s"/>
    </signal>
    <signal name="RecordingError">
      <arg name="error" type="s"/>
    </signal>
  </interface>
</node>`;

const VoicifyProxy = Gio.DBusProxy.makeProxyWrapper(VoicifyDBusInterface);

export default class VoicifyExtension extends Extension {
    constructor(metadata) {
        super(metadata);
        this._indicator = null;
        this._icon = null;
        this._action = null;
        this._state = State.IDLE;
        this._timeoutId = null;
        this._waveWidget = null;
        this._waveBars = null;
        this._barTimers = null;
        this._uploadTimer = null;
        this._finishedTimer = null;
        this._dbusProxy = null;
        this._lastShortcutTime = 0;
        this._debounceMs = 500; // Prevent multiple calls within 500ms
    }

    enable() {
        console.debug('Voicify extension enabled');

        // Initialize D-Bus proxy
        this._initDBusProxy();

        // Create panel indicator
        this._createIndicator();

        // Set up global shortcut
        this._setupGlobalShortcut();
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

        // Clean up global shortcut
        if (this._action !== null) {
            global.display.ungrab_accelerator(this._action);
            Main.wm.allowKeybinding(
                Meta.external_binding_name_for_action(this._action),
                Shell.ActionMode.NONE
            );
            this._action = null;
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
        this._indicator.connect('button-press-event', () => this._onShortcutPressed());

        // Add to panel
        Main.panel.addToStatusArea(this.uuid, this._indicator);
    }

    _setupGlobalShortcut() {
        this._action = global.display.grab_accelerator(SHORTCUT_KEY, Meta.KeyBindingFlags.NONE);

        if (this._action == Meta.KeyBindingAction.NONE) {
            console.error('Unable to grab accelerator for Voicify');
            return;
        }

        const name = Meta.external_binding_name_for_action(this._action);
        Main.wm.allowKeybinding(name, Shell.ActionMode.ALL);

        // Connect to accelerator activated signal
        global.display.connect('accelerator-activated', (display, action, deviceId, timestamp) => {
            if (action === this._action) {
                this._onShortcutPressed();
            }
        });

        console.debug('Global shortcut registered:', SHORTCUT_KEY);
    }

    _onShortcutPressed() {
        const currentTime = Date.now();

        // Debounce: ignore if called too quickly after the last call
        if (currentTime - this._lastShortcutTime < this._debounceMs) {
            console.debug('🔥 SHORTCUT DEBOUNCED - ignoring rapid call');
            return;
        }
        this._lastShortcutTime = currentTime;

        console.log('🔥 SHORTCUT PRESSED! Current state:', this._state);

        switch (this._state) {
            case State.IDLE:
                this._startRecording();
                break;
            case State.RECORDING:
                this._stopRecording();
                break;
            case State.UPLOADING:
            case State.FINISHED:
                // Ignore - already processing
                console.debug('Already processing - ignoring shortcut');
                break;
        }
    }

    _startRecording() {
        console.log('🔥 _startRecording() called - calling D-Bus ToggleRecording');

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        // Call D-Bus method to toggle recording
        this._dbusProxy.ToggleRecordingAsync()
            .then(() => {
                console.debug('D-Bus: ToggleRecording method called successfully');
                // State will be updated via RecordingStarted signal
            })
            .catch(error => {
                console.error('D-Bus: Failed to call ToggleRecording:', error);
                // Reset state on error
                this._state = State.IDLE;
                this._updateIndicator();
            });
    }

    _stopRecording() {
        console.log('🔥 _stopRecording() called - calling D-Bus ToggleRecording');

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        // Switch to uploading state immediately
        this._state = State.UPLOADING;
        this._updateIndicator();
        this._updateWaveWidget();

        // Call D-Bus method to stop recording
        this._dbusProxy.ToggleRecordingAsync()
            .then(() => {
                console.debug('D-Bus: ToggleRecording method called successfully');
                // State will be updated via TranscriptionReady signal
            })
            .catch(error => {
                console.error('D-Bus: Failed to call ToggleRecording:', error);
                // Reset state on error
                this._state = State.IDLE;
                this._updateIndicator();
                this._hideWaveWidget();
            });
    }

    _onTranscriptionReady(text) {
        this._state = State.FINISHED;
        this._updateIndicator();
        this._startFinishedAnimation();
        console.debug('Transcription ready:', text);

        // Copy to clipboard and auto-paste (extension handles all text injection)
        St.Clipboard.get_default().set_text(St.ClipboardType.CLIPBOARD, text);
        console.debug('Text copied to clipboard:', text);

        // Auto-paste with virtual keyboard (X11 only)
        this._performAutoPaste();
    }

    _performAutoPaste() {
        try {
            // Try virtual keyboard paste (works on X11)
            const seat = Clutter.get_default_backend().get_default_seat();
            const virtualKeyboard = seat.create_virtual_device(Clutter.InputDeviceType.KEYBOARD_DEVICE);

            const eventTime = global.get_current_time();

            // Send Ctrl+V
            virtualKeyboard.notify_keyval(eventTime, Clutter.KEY_Control_L, Clutter.KeyState.PRESSED);
            virtualKeyboard.notify_keyval(eventTime + 10, Clutter.KEY_v, Clutter.KeyState.PRESSED);
            virtualKeyboard.notify_keyval(eventTime + 20, Clutter.KEY_v, Clutter.KeyState.RELEASED);
            virtualKeyboard.notify_keyval(eventTime + 30, Clutter.KEY_Control_L, Clutter.KeyState.RELEASED);

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
            reactive: false,
            can_focus: false,
            track_hover: false,
            visible: true,
        });

        // Create wave container
        const waveContainer = new St.BoxLayout({
            style_class: 'voicify-wave-container',
            vertical: false,
            x_align: Clutter.ActorAlign.CENTER,
            y_align: Clutter.ActorAlign.CENTER,
        });

        // Create equalizer bars
        this._waveBars = [];
        for (let i = 0; i < 10; i++) {
            const bar = new St.Widget({
                style_class: `voicify-wave-bar`,
                width: 4,
                height: 15, // Increased by 50%
                visible: true,
            });
            this._waveBars.push(bar);
            waveContainer.add_child(bar);
            console.log(`🔥 Created bar ${i} with height: ${bar.height}`);
        }

        // Set fixed container size - increased by 50%
        waveContainer.set_size(75, 30);

        this._waveWidget.add_child(waveContainer);

        // Add to main UI group (overlay)
        Main.uiGroup.add_child(this._waveWidget);

        // Position at bottom left of center  
        const monitor = Main.layoutManager.primaryMonitor;
        this._waveWidget.set_position(
            monitor.x + monitor.width / 2, // Move left
            monitor.y + monitor.height * 0.98 - 20  // More to bottom
        );

        console.debug(`Wave widget positioned at: ${monitor.x + monitor.width / 2 - 60}, ${monitor.y + monitor.height * 0.98 - 12}`);

        // Start recording animation
        this._startWaveAnimation();
    }

    _updateWaveWidget() {
        if (!this._waveWidget) return;

        // Change to upload animation
        this._waveWidget.style_class = 'voicify-wave-overlay uploading';
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

        if (!this._waveWidget || !this._waveBars) return;

        // Change widget class to finished
        this._waveWidget.style_class = 'voicify-wave-overlay finished';

        console.debug('Starting finished animation - start at 100% then shrink to 0');

        // Set all bars to 100% immediately
        this._waveBars.forEach(bar => {
            bar.scale_y = 1.2; // Start at max scale
        });

        // Short pause then animate down to 0
        this._finishedTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 150, () => {
            this._finishedTimer = null;

            let phase = 0;
            this._finishedTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 30, () => {
                // Animate all bars down to 0 simultaneously
                const progress = phase / 25;
                const scale = 1.2 * (1 - progress); // Shrink from max to 0

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

    _startWaveAnimation() {
        if (!this._waveBars) return;

        console.debug('Starting JavaScript equalizer animation');

        // Each bar gets its own animation timer
        this._barTimers = [];

        this._waveBars.forEach((bar, index) => {
            const minScale = 0.2;
            const maxScale = 0.7 + Math.random() * 0.7; // SMALLER max scale  
            const speed = 30 + Math.random() * 60;      // 15% slower speed

            let scale = minScale;
            let direction = 1;

            // Set pivot point to bottom for scaling from bottom up
            bar.set_pivot_point(0.5, 1.0); // X center, Y bottom

            console.log(`🔥 Bar ${index}: speed=${speed}, maxScale=${maxScale.toFixed(2)}`);

            const timer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, speed, () => {
                if (this._state !== State.RECORDING) {
                    return GLib.SOURCE_REMOVE;
                }

                // Animate scale up and down
                scale += direction * (0.1 + Math.random() * 0.2);

                if (scale >= maxScale) {
                    scale = maxScale;
                    direction = -1;
                } else if (scale <= minScale) {
                    scale = minScale;
                    direction = 1;
                }

                // Scale from bottom up using pivot point
                bar.scale_y = scale;

                return GLib.SOURCE_CONTINUE;
            });

            this._barTimers.push(timer);
        });
    }

    _startUploadAnimation() {
        // Stop recording animations first
        if (this._barTimers) {
            this._barTimers.forEach(timer => GLib.Source.remove(timer));
            this._barTimers = [];
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

                bar.scale_y = Math.max(0.2, Math.min(1.2, scale));
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
            console.debug('D-Bus: Recording started signal received');
            this._state = State.RECORDING;
            this._updateIndicator();
            this._showWaveWidget();
        });

        this._dbusProxy.connectSignal('TranscriptionReady', (proxy, sender, [text]) => {
            console.debug('D-Bus: Transcription ready signal received:', text);
            this._onTranscriptionReady(text);
        });

        this._dbusProxy.connectSignal('RecordingError', (proxy, sender, [error]) => {
            console.error('D-Bus: Recording error signal received:', error);
            this._state = State.IDLE;
            this._updateIndicator();
            this._hideWaveWidget();
        });

        console.debug('D-Bus proxy initialized');
    }
}