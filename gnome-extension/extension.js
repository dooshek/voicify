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
import * as PopupMenu from 'resource:///org/gnome/shell/ui/popupMenu.js';

// Size variants
const SIZES = {
    'thin':   { barWidth: 2, barSpacing: 2, numBars: 32, containerHeight: 40 },
    'medium': { barWidth: 4, barSpacing: 3, numBars: 26, containerHeight: 56 },
    'large':  { barWidth: 6, barSpacing: 4, numBars: 20, containerHeight: 72 },
};

const MAX_SCALE = 1.0;
const MIN_SCALE = 0.04;

// Background padding for wave container (matches CSS padding in stylesheet.css)
const WAVE_H_PAD = 14;
const WAVE_V_PAD = 10;

// Recording states
const State = {
    IDLE: 'idle',
    RECORDING: 'recording',
    UPLOADING: 'uploading',
    FINISHED: 'finished',
    CANCELED: 'canceled'
};

// Waveform color themes
const THEMES = {
    'mint-dream':  { name: 'Mint Dream',  center: {r: 94,  g: 240, b: 218}, edge: {r: 70,  g: 160, b: 255} },
    'ocean':       { name: 'Ocean',       center: {r: 0,   g: 229, b: 255}, edge: {r: 83,  g: 109, b: 254} },
    'sunset':      { name: 'Sunset',      center: {r: 255, g: 171, b: 64},  edge: {r: 255, g: 64,  b: 129} },
    'aurora':      { name: 'Aurora',       center: {r: 105, g: 240, b: 174}, edge: {r: 224, g: 64,  b: 251} },
    'coral':       { name: 'Coral',        center: {r: 255, g: 138, b: 128}, edge: {r: 234, g: 128, b: 252} },
    'deep-sea':    { name: 'Deep Sea',    center: {r: 38,  g: 166, b: 154}, edge: {r: 21,  g: 101, b: 192} },
    'forest':      { name: 'Forest',      center: {r: 102, g: 187, b: 106}, edge: {r: 141, g: 110, b: 99}  },
    'ember':       { name: 'Ember',       center: {r: 239, g: 83,  b: 80},  edge: {r: 255, g: 143, b: 0}   },
    'twilight':    { name: 'Twilight',    center: {r: 179, g: 157, b: 219}, edge: {r: 92,  g: 107, b: 192} },
    'graphite':    { name: 'Graphite',    center: {r: 189, g: 189, b: 189}, edge: {r: 120, g: 144, b: 156} },
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

// D-Bus service configuration for on-demand daemon activation
const DBUS_SERVICE_NAME = 'com.dooshek.voicify';
const VOICIFY_BINARY_PATH = GLib.get_home_dir() + '/bin/voicify';
const VOICIFY_LOG_PATH = GLib.get_home_dir() + '/.config/voicify/voicify.log';

export default class VoicifyExtension extends Extension {
    enable() {
        console.debug('Voicify extension enabled');

        this._indicator = null;
        this._icon = null;
        this._cancelAction = null;
        this._realtimeAction = null;
        this._postAutoPasteAction = null;
        this._postRouterAction = null;
        this._acceleratorHandlerId = 0;
        this._state = State.IDLE;
        this._waveWidget = null;
        this._waveBars = null;
        this._uploadTimer = null;
        this._finishedTimer = null;
        this._pasteTimerId = null;
        this._dbusProxy = null;
        this._dbusSignalIds = [];
        this._currentLevel = 0;
        this._levelTimer = null;
        this._lastShortcutTime = 0;
        this._isRealtimeMode = false;
        this._isPostAutoPaste = false;
        this._isPostRouter = false;
        this._debounceMs = 500;
        this._virtualKeyboard = null;
        this._settingsChangedIds = [];
        this._currentTheme = THEMES['mint-dream'];
        this._currentSize = SIZES['medium'];
        this._bgOpacity = 0.25;
        this._waveContainer = null;
        this._currentPosition = 'bottom-center';
        this._reactionTime = 10;
        this._smoothingAlpha = 0.5;
        this._sensitivityGain = 1.0;
        this._menuShortcutLabels = {};

        this._settings = this.getSettings();

        // Apply initial settings
        this._applyTheme();
        this._applySize();
        this._applyPosition();
        this._applyReactionTime();
        this._applySmoothing();
        this._applySensitivity();
        this._applyBgOpacity();

        // Connect settings change handlers
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-theme', () => this._applyTheme())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-size', () => this._applySize())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-position', () => this._applyPosition())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::reaction-time', () => this._applyReactionTime())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-smoothing', () => this._applySmoothing())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-sensitivity', () => this._applySensitivity())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-bg-opacity', () => this._applyBgOpacity())
        );

        // Shortcut change handlers
        this._settingsChangedIds.push(
            this._settings.connect('changed::shortcut-cancel', () => {
                this._grabShortcut('shortcut-cancel', '_cancelAction');
                this._updateMenuShortcutLabel('shortcut-cancel');
            })
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::shortcut-realtime', () => {
                this._grabShortcut('shortcut-realtime', '_realtimeAction');
                this._updateMenuShortcutLabel('shortcut-realtime');
            })
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::shortcut-post-autopaste', () => {
                this._grabShortcut('shortcut-post-autopaste', '_postAutoPasteAction');
                this._updateMenuShortcutLabel('shortcut-post-autopaste');
            })
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::shortcut-post-router', () => {
                this._grabShortcut('shortcut-post-router', '_postRouterAction');
                this._updateMenuShortcutLabel('shortcut-post-router');
            })
        );

        this._ensureDBusServiceFile();

        try {
            const seat = Clutter.get_default_backend().get_default_seat();
            this._virtualKeyboard = seat.create_virtual_device(Clutter.InputDeviceType.KEYBOARD_DEVICE);
        } catch (error) {
            console.debug('Failed to create virtual keyboard device:', error.message);
            this._virtualKeyboard = null;
        }

        this._initDBusProxy();
        this._createIndicator();

        // Grab shortcuts from settings
        this._grabShortcut('shortcut-cancel', '_cancelAction');
        this._grabShortcut('shortcut-realtime', '_realtimeAction');
        this._grabShortcut('shortcut-post-autopaste', '_postAutoPasteAction');
        this._grabShortcut('shortcut-post-router', '_postRouterAction');

        // Single accelerator-activated handler for all shortcuts
        this._acceleratorHandlerId = global.display.connect('accelerator-activated',
            (display, action) => {
                if (action === this._cancelAction) this._onCancelShortcutPressed();
                else if (action === this._realtimeAction) this._onRealtimeShortcutPressed();
                else if (action === this._postAutoPasteAction) this._onPostAutoPasteShortcutPressed();
                else if (action === this._postRouterAction) this._onPostRouterShortcutPressed();
            });
    }

    disable() {
        console.debug('Voicify extension disabled');

        // Disconnect settings handlers
        if (this._settings) {
            for (const id of this._settingsChangedIds) {
                this._settings.disconnect(id);
            }
            this._settingsChangedIds = [];
        }
        this._settings = null;

        // Disconnect accelerator handler
        if (this._acceleratorHandlerId) {
            global.display.disconnect(this._acceleratorHandlerId);
            this._acceleratorHandlerId = 0;
        }

        this._cleanupAnimationTimers();

        this._ungrabShortcut('_cancelAction');
        this._ungrabShortcut('_realtimeAction');
        this._ungrabShortcut('_postAutoPasteAction');
        this._ungrabShortcut('_postRouterAction');

        if (this._waveWidget) {
            this._waveWidget.destroy();
            this._waveWidget = null;
            this._waveContainer = null;
            this._waveBars = null;
        }

        if (this._dbusProxy) {
            for (const id of this._dbusSignalIds) {
                this._dbusProxy.disconnectSignal(id);
            }
            this._dbusSignalIds = [];
            this._dbusProxy = null;
        }

        this._menuShortcutLabels = {};

        if (this._indicator) {
            this._indicator.destroy();
            this._indicator = null;
        }
        this._icon = null;
        this._virtualKeyboard = null;
    }

    // --- Timer management ---

    _cleanupAnimationTimers() {
        if (this._pasteTimerId) {
            GLib.Source.remove(this._pasteTimerId);
            this._pasteTimerId = null;
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
    }

    // --- Panel indicator ---

    _createIndicator() {
        this._indicator = new PanelMenu.Button(0.5, this.metadata.name, false);

        const iconPath = GLib.build_filenamev([this.path, 'icons', 'voicify-symbolic.svg']);
        this._icon = new St.Icon({
            gicon: Gio.icon_new_for_string(iconPath),
            style_class: 'system-status-icon',
        });

        this._indicator.add_child(this._icon);

        // Recording mode menu items with shortcut labels
        this._addModeMenuItem('Realtime', 'shortcut-realtime',
            () => this._onRealtimeShortcutPressed());
        this._addModeMenuItem('Post + auto-paste', 'shortcut-post-autopaste',
            () => this._onPostAutoPasteShortcutPressed());
        this._addModeMenuItem('Post + router', 'shortcut-post-router',
            () => this._onPostRouterShortcutPressed());
        this._addModeMenuItem('Cancel', 'shortcut-cancel',
            () => this._onCancelShortcutPressed());

        this._indicator.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        const settingsItem = new PopupMenu.PopupMenuItem('Settings');
        settingsItem.connect('activate', () => this.openPreferences());
        this._indicator.menu.addMenuItem(settingsItem);

        Main.panel.addToStatusArea(this.uuid, this._indicator);
    }

    _addModeMenuItem(label, settingKey, callback) {
        const item = new PopupMenu.PopupMenuItem(label);

        const shortcutLabel = new St.Label({
            text: this._formatAccelerator(this._settings.get_string(settingKey)),
            x_expand: true,
            x_align: Clutter.ActorAlign.END,
            style: 'font-size: 0.85em; color: rgba(255,255,255,0.5); margin-left: 20px;',
        });
        item.add_child(shortcutLabel);
        this._menuShortcutLabels[settingKey] = shortcutLabel;

        item.connect('activate', callback);
        this._indicator.menu.addMenuItem(item);
    }

    _updateMenuShortcutLabel(settingKey) {
        const label = this._menuShortcutLabels[settingKey];
        if (label && this._settings) {
            label.text = this._formatAccelerator(this._settings.get_string(settingKey));
        }
    }

    _formatAccelerator(accel) {
        if (!accel) return '';
        let result = accel;
        result = result.replace(/<Primary>/gi, 'Ctrl+');
        result = result.replace(/<Control>/gi, 'Ctrl+');
        result = result.replace(/<Ctrl>/gi, 'Ctrl+');
        result = result.replace(/<Shift>/gi, 'Shift+');
        result = result.replace(/<Alt>/gi, 'Alt+');
        result = result.replace(/<Super>/gi, 'Super+');
        result = result.replace(/<Meta>/gi, 'Meta+');
        const parts = result.split('+');
        if (parts.length > 0) {
            parts[parts.length - 1] = parts[parts.length - 1].toUpperCase();
        }
        return parts.join('+');
    }

    _updateIndicator() {
        if (!this._icon) return;

        if (this._state === State.RECORDING) {
            const { r, g, b } = this._currentTheme.center;
            this._icon.style = `color: rgb(${r},${g},${b});`;
        } else if (this._state === State.UPLOADING) {
            const { center } = this._computeStateColors(State.UPLOADING);
            this._icon.style = `color: rgb(${center.r},${center.g},${center.b});`;
        } else {
            this._icon.style = null;
        }
    }

    // --- Settings apply methods ---

    _applyTheme() {
        const themeId = this._settings
            ? this._settings.get_string('wave-theme')
            : 'mint-dream';
        this._currentTheme = THEMES[themeId] || THEMES['mint-dream'];
        this._updateBarColors();
    }

    _applySize() {
        const sizeId = this._settings
            ? this._settings.get_string('wave-size')
            : 'medium';
        this._currentSize = SIZES[sizeId] || SIZES['medium'];

        if (this._waveWidget && this._state === State.RECORDING) {
            this._hideWaveWidget();
            this._showWaveWidget();
            this._startLevelWave();
        }
    }

    _applyPosition() {
        this._currentPosition = this._settings
            ? this._settings.get_string('wave-position')
            : 'bottom-center';

        if (this._waveWidget) {
            const { barWidth, barSpacing, numBars, containerHeight } = this._currentSize;
            const containerWidth = numBars * (barWidth + barSpacing) + WAVE_H_PAD * 2;
            const totalHeight = containerHeight + WAVE_V_PAD * 2;
            this._positionWaveWidget(containerWidth, totalHeight);
        }
    }

    _applyReactionTime() {
        const val = this._settings
            ? this._settings.get_int('reaction-time')
            : 10;
        this._reactionTime = Math.max(5, Math.min(40, val));

        if (this._state === State.RECORDING && this._levelTimer) {
            this._stopLevelWave();
            this._startLevelWave();
        }
    }

    _applySmoothing() {
        const val = this._settings
            ? this._settings.get_int('wave-smoothing')
            : 50;
        // 0 = jumpy (alpha=1.0), 100 = very smooth (alpha=0.05)
        this._smoothingAlpha = Math.max(0.05, 1.0 - (val / 100) * 0.95);
    }

    _applySensitivity() {
        const val = this._settings
            ? this._settings.get_int('wave-sensitivity')
            : 100;
        this._sensitivityGain = val / 100;
    }

    _applyBgOpacity() {
        const val = this._settings
            ? this._settings.get_int('wave-bg-opacity')
            : 25;
        this._bgOpacity = val / 100;

        if (this._waveContainer) {
            this._waveContainer.style = `background-color: rgba(0, 0, 0, ${this._bgOpacity.toFixed(2)});`;
        }
    }

    _updateBarColors(colors = null) {
        if (!this._waveBars) return;

        const { center, edge } = colors || this._currentTheme;
        const { barWidth, barSpacing } = this._currentSize;
        const barCenter = (this._waveBars.length - 1) / 2;
        const barHeight = (this._currentSize.containerHeight / 2) + 1;

        for (let i = 0; i < this._waveBars.length; i++) {
            const isLast = (i === this._waveBars.length - 1);
            const dist = Math.abs(i - barCenter) / barCenter;
            const r = Math.round(center.r + (edge.r - center.r) * dist);
            const g = Math.round(center.g + (edge.g - center.g) * dist);
            const b = Math.round(center.b + (edge.b - center.b) * dist);
            this._waveBars[i].style = `width: ${barWidth}px; height: ${barHeight}px; background-color: rgb(${r},${g},${b});${isLast ? '' : ` margin-right: ${barSpacing}px;`}`;
        }
    }

    _blendColor(a, b, t) {
        return {
            r: Math.round(a.r + (b.r - a.r) * t),
            g: Math.round(a.g + (b.g - a.g) * t),
            b: Math.round(a.b + (b.b - a.b) * t),
        };
    }

    _rgbToHsl(r, g, b) {
        r /= 255; g /= 255; b /= 255;
        const max = Math.max(r, g, b), min = Math.min(r, g, b);
        let h, s, l = (max + min) / 2;

        if (max === min) {
            h = s = 0;
        } else {
            const d = max - min;
            s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
            if (max === r) h = ((g - b) / d + (g < b ? 6 : 0)) / 6;
            else if (max === g) h = ((b - r) / d + 2) / 6;
            else h = ((r - g) / d + 4) / 6;
        }
        return { h, s, l };
    }

    _hslToRgb(h, s, l) {
        if (s === 0) {
            const v = Math.round(l * 255);
            return { r: v, g: v, b: v };
        }
        const hue2rgb = (p, q, t) => {
            if (t < 0) t += 1;
            if (t > 1) t -= 1;
            if (t < 1 / 6) return p + (q - p) * 6 * t;
            if (t < 1 / 2) return q;
            if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
            return p;
        };
        const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
        const p = 2 * l - q;
        return {
            r: Math.round(hue2rgb(p, q, h + 1 / 3) * 255),
            g: Math.round(hue2rgb(p, q, h) * 255),
            b: Math.round(hue2rgb(p, q, h - 1 / 3) * 255),
        };
    }

    _rotateHue(color, degrees) {
        const hsl = this._rgbToHsl(color.r, color.g, color.b);
        hsl.h = (hsl.h + degrees / 360 + 1) % 1;
        hsl.s = Math.min(1, hsl.s * 1.15);
        return this._hslToRgb(hsl.h, hsl.s, hsl.l);
    }

    _computeStateColors(state) {
        const { center, edge } = this._currentTheme;

        if (state === State.UPLOADING) {
            return {
                center: this._rotateHue(center, 120),
                edge: this._rotateHue(edge, 120),
            };
        }

        return { center, edge };
    }

    // --- Keyboard shortcuts ---

    _grabShortcut(settingKey, actionField) {
        // Ungrab previous if exists
        this._ungrabShortcut(actionField);

        const accel = this._settings.get_string(settingKey);
        if (!accel) return;

        const action = global.display.grab_accelerator(accel, Meta.KeyBindingFlags.NONE);
        if (action === Meta.KeyBindingAction.NONE) {
            console.error('Unable to grab accelerator:', accel);
            return;
        }

        const name = Meta.external_binding_name_for_action(action);
        Main.wm.allowKeybinding(name, Shell.ActionMode.ALL);
        this[actionField] = action;
    }

    _ungrabShortcut(actionField) {
        if (this[actionField] !== null) {
            global.display.ungrab_accelerator(this[actionField]);
            Main.wm.allowKeybinding(
                Meta.external_binding_name_for_action(this[actionField]),
                Shell.ActionMode.NONE
            );
            this[actionField] = null;
        }
    }

    // --- Shortcut handlers ---

    _onCancelShortcutPressed() {
        if (this._state === State.IDLE) return;

        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._dbusProxy.CancelRecordingAsync()
            .then(() => {
                console.debug('D-Bus: CancelRecording called');
                this._state = State.IDLE;
                this._isRealtimeMode = false;
                this._isPostAutoPaste = false;
                this._isPostRouter = false;
                this._updateIndicator();
                this._hideWaveWidget();
            })
            .catch(error => {
                console.error('D-Bus: Failed to call CancelRecording:', error);
                this._state = State.IDLE;
                this._isRealtimeMode = false;
                this._isPostAutoPaste = false;
                this._isPostRouter = false;
                this._updateIndicator();
                this._hideWaveWidget();
            });
    }

    _onRealtimeShortcutPressed() {
        if (Date.now() - this._lastShortcutTime < this._debounceMs) return;
        this._lastShortcutTime = Date.now();

        if (this._state === State.IDLE) {
            this._startRealtimeRecording();
        } else if (this._state === State.RECORDING && this._isRealtimeMode) {
            this._stopRealtimeRecording();
        }
    }

    _onPostAutoPasteShortcutPressed() {
        if (Date.now() - this._lastShortcutTime < this._debounceMs) return;
        this._lastShortcutTime = Date.now();

        if (this._state === State.IDLE) {
            this._startPostAutoPasteRecording();
        } else if (this._state === State.RECORDING && this._isPostAutoPaste) {
            this._stopPostAutoPasteRecording();
        }
    }

    _onPostRouterShortcutPressed() {
        if (Date.now() - this._lastShortcutTime < this._debounceMs) return;
        this._lastShortcutTime = Date.now();

        if (this._state === State.IDLE) {
            this._startPostRouterRecording();
        } else if (this._state === State.RECORDING && this._isPostRouter) {
            this._stopPostRouterRecording();
        }
    }

    // --- Recording start/stop ---

    _startRealtimeRecording() {
        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();
        this._isRealtimeMode = true;
        this._isPostAutoPaste = false;
        this._isPostRouter = false;

        this._dbusProxy.StartRealtimeRecordingAsync()
            .then(() => console.debug('D-Bus: StartRealtimeRecording called'))
            .catch(error => {
                console.error('D-Bus: Failed to call StartRealtimeRecording:', error);
                this._state = State.IDLE;
                this._isRealtimeMode = false;
                this._updateIndicator();
            });
    }

    _startPostAutoPasteRecording() {
        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();
        this._isRealtimeMode = false;
        this._isPostAutoPaste = true;
        this._isPostRouter = false;

        this._dbusProxy.TogglePostTranscriptionAutoPasteAsync()
            .then(() => console.debug('D-Bus: TogglePostTranscriptionAutoPaste called'))
            .catch(error => {
                console.error('D-Bus: Failed to call TogglePostTranscriptionAutoPaste:', error);
                this._state = State.IDLE;
                this._isPostAutoPaste = false;
                this._updateIndicator();
            });
    }

    _startPostRouterRecording() {
        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();
        this._isRealtimeMode = false;
        this._isPostAutoPaste = false;
        this._isPostRouter = true;

        this._dbusProxy.TogglePostTranscriptionRouterAsync()
            .then(() => console.debug('D-Bus: TogglePostTranscriptionRouter called'))
            .catch(error => {
                console.error('D-Bus: Failed to call TogglePostTranscriptionRouter:', error);
                this._state = State.IDLE;
                this._isPostRouter = false;
                this._updateIndicator();
            });
    }

    _stopRealtimeRecording() {
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
        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();
        this._state = State.UPLOADING;
        this._updateIndicator();
        this._updateWaveWidget();

        this._dbusProxy.TogglePostTranscriptionAutoPasteAsync()
            .then(() => console.debug('D-Bus: TogglePostTranscriptionAutoPaste called (stop)'))
            .catch(error => {
                console.error('D-Bus: Failed to call TogglePostTranscriptionAutoPaste:', error);
                this._state = State.IDLE;
                this._isPostAutoPaste = false;
                this._updateIndicator();
                this._hideWaveWidget();
            });
    }

    _stopPostRouterRecording() {
        if (!this._dbusProxy) {
            console.error('D-Bus proxy not initialized');
            return;
        }

        this._updateFocusedWindowInDaemon();
        this._state = State.UPLOADING;
        this._updateIndicator();
        this._updateWaveWidget();

        this._dbusProxy.TogglePostTranscriptionRouterAsync()
            .then(() => console.debug('D-Bus: TogglePostTranscriptionRouter called (stop)'))
            .catch(error => {
                console.error('D-Bus: Failed to call TogglePostTranscriptionRouter:', error);
                this._state = State.IDLE;
                this._isPostRouter = false;
                this._updateIndicator();
                this._hideWaveWidget();
            });
    }

    // --- D-Bus signal handlers ---

    _onTranscriptionReady(text) {
        console.debug('Transcription ready:', text);

        if (this._isRealtimeMode) return;

        if (this._isPostRouter) {
            console.debug('Post-router mode - waiting for plugin RequestPaste signal');
            this._state = State.FINISHED;
            this._isPostRouter = false;
            this._updateIndicator();
            this._startFinishedAnimation();
        } else if (this._isPostAutoPaste) {
            this._performAutoPaste();
            this._state = State.FINISHED;
            this._isPostAutoPaste = false;
            this._updateIndicator();
            this._startFinishedAnimation();
        }
    }

    _onPartialTranscription(text) {
        console.debug('Partial transcription:', text);
    }

    _onCompleteTranscription(text) {
        if (this._isRealtimeMode && text && text.length > 0) {
            this._injectTextDelta(text + ' ');
        }
    }

    _onRequestPaste(text) {
        console.debug('RequestPaste from plugin');
        St.Clipboard.get_default().set_text(St.ClipboardType.CLIPBOARD, text);

        if (this._pasteTimerId) GLib.Source.remove(this._pasteTimerId);
        this._pasteTimerId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 100, () => {
            this._pasteTimerId = null;
            this._performAutoPaste();
            return GLib.SOURCE_REMOVE;
        });
    }

    _onRecordingCancelled() {
        this._state = State.IDLE;
        this._isRealtimeMode = false;
        this._isPostAutoPaste = false;
        this._isPostRouter = false;
        this._updateIndicator();
        this._hideWaveWidget();
    }

    // --- Text injection ---

    _injectTextDelta(text) {
        St.Clipboard.get_default().set_text(St.ClipboardType.CLIPBOARD, text);

        if (this._pasteTimerId) GLib.Source.remove(this._pasteTimerId);
        this._pasteTimerId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 100, () => {
            this._pasteTimerId = null;
            this._performAutoPaste();
            return GLib.SOURCE_REMOVE;
        });
    }

    _performAutoPaste() {
        try {
            if (!this._virtualKeyboard) return;

            const eventTime = global.get_current_time();
            this._virtualKeyboard.notify_keyval(eventTime, Clutter.KEY_Control_L, Clutter.KeyState.PRESSED);
            this._virtualKeyboard.notify_keyval(eventTime + 10, Clutter.KEY_v, Clutter.KeyState.PRESSED);
            this._virtualKeyboard.notify_keyval(eventTime + 20, Clutter.KEY_v, Clutter.KeyState.RELEASED);
            this._virtualKeyboard.notify_keyval(eventTime + 30, Clutter.KEY_Control_L, Clutter.KeyState.RELEASED);
        } catch (error) {
            console.debug('Virtual keyboard paste failed:', error.message);
        }
    }

    // --- Wave visualization widget ---

    _showWaveWidget() {
        if (this._waveWidget) return;

        const { barWidth, barSpacing, numBars, containerHeight } = this._currentSize;
        const containerWidth = numBars * (barWidth + barSpacing);
        const barHeight = (containerHeight / 2) + 1;

        this._waveWidget = new St.Widget({
            style_class: 'voicify-wave-overlay',
            reactive: true,
            can_focus: false,
            track_hover: false,
            visible: true,
        });

        this._waveContainer = new St.BoxLayout({
            style_class: 'voicify-wave-container',
            style: `background-color: rgba(0, 0, 0, ${this._bgOpacity.toFixed(2)});`,
            vertical: false,
            x_align: Clutter.ActorAlign.CENTER,
            y_align: Clutter.ActorAlign.CENTER,
            clip_to_allocation: true,
        });

        this._waveBars = [];
        const { center, edge } = this._currentTheme;
        const barCenter = (numBars - 1) / 2;
        for (let i = 0; i < numBars; i++) {
            const isLast = (i === numBars - 1);
            const dist = Math.abs(i - barCenter) / barCenter;
            const r = Math.round(center.r + (edge.r - center.r) * dist);
            const g = Math.round(center.g + (edge.g - center.g) * dist);
            const b = Math.round(center.b + (edge.b - center.b) * dist);
            const barOpacity = Math.round(80 + 175 * (1 - dist));
            const bar = new St.Widget({
                style_class: 'voicify-wave-bar',
                style: `width: ${barWidth}px; height: ${barHeight}px; background-color: rgb(${r},${g},${b});${isLast ? '' : ` margin-right: ${barSpacing}px;`}`,
                visible: true,
                clip_to_allocation: true,
                opacity: barOpacity,
            });
            this._waveBars.push(bar);
            this._waveContainer.add_child(bar);
        }

        const totalWidth = containerWidth + WAVE_H_PAD * 2;
        const totalHeight = containerHeight + WAVE_V_PAD * 2;
        this._waveContainer.set_size(totalWidth, totalHeight);
        this._waveWidget.add_child(this._waveContainer);

        Main.layoutManager.addChrome(this._waveWidget);
        this._positionWaveWidget(totalWidth, totalHeight);
    }

    _positionWaveWidget(containerWidth, containerHeight) {
        if (!this._waveWidget) return;

        const monitor = Main.layoutManager.primaryMonitor;
        const margin = 20;
        const panelHeight = Main.panel.height;
        let x, y;

        const middleY = monitor.y + (monitor.height - containerHeight) / 2;

        switch (this._currentPosition) {
            case 'top-left':
                x = monitor.x + margin;
                y = monitor.y + panelHeight + margin;
                break;
            case 'top-center':
                x = monitor.x + (monitor.width - containerWidth) / 2;
                y = monitor.y + panelHeight + margin;
                break;
            case 'top-right':
                x = monitor.x + monitor.width - containerWidth - margin;
                y = monitor.y + panelHeight + margin;
                break;
            case 'middle-left':
                x = monitor.x + margin;
                y = middleY;
                break;
            case 'middle-center':
                x = monitor.x + (monitor.width - containerWidth) / 2;
                y = middleY;
                break;
            case 'middle-right':
                x = monitor.x + monitor.width - containerWidth - margin;
                y = middleY;
                break;
            case 'bottom-left':
                x = monitor.x + margin;
                y = monitor.y + monitor.height - containerHeight - margin;
                break;
            case 'bottom-center':
                x = monitor.x + (monitor.width - containerWidth) / 2;
                y = monitor.y + monitor.height - containerHeight - margin;
                break;
            case 'bottom-right':
                x = monitor.x + monitor.width - containerWidth - margin;
                y = monitor.y + monitor.height - containerHeight - margin;
                break;
            default:
                x = monitor.x + (monitor.width - containerWidth) / 2;
                y = monitor.y + monitor.height - containerHeight - margin;
                break;
        }

        this._waveWidget.set_position(x, y);
    }

    _updateWaveWidget() {
        if (!this._waveWidget) return;

        this._waveWidget.style_class = 'voicify-wave-overlay uploading';
        this._stopLevelWave();
        this._startUploadAnimation();
    }

    _hideWaveWidget() {
        this._cleanupAnimationTimers();

        if (this._waveWidget) {
            this._waveWidget.destroy();
            this._waveWidget = null;
            this._waveContainer = null;
            this._waveBars = null;
        }
    }

    // --- Visualization: recording level ---

    _initFlatBars() {
        if (!this._waveBars) return;
        this._currentLevel = 0;
        this._waveBars.forEach(bar => {
            bar.set_pivot_point(0.5, 0.5);
            bar.scale_y = MIN_SCALE;
        });
    }

    _pushLevel(level) {
        if (!this._waveBars) return;
        const amplified = Math.min(MAX_SCALE, level * this._sensitivityGain);
        const raw = Math.max(MIN_SCALE, amplified);
        this._currentLevel = this._smoothingAlpha * raw + (1 - this._smoothingAlpha) * (this._currentLevel || 0);
    }

    _startLevelWave() {
        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }
        this._initFlatBars();

        this._levelTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, this._reactionTime, () => {
            if (this._state !== State.RECORDING || !this._waveBars) {
                this._levelTimer = null;
                return GLib.SOURCE_REMOVE;
            }

            const n = this._waveBars.length;
            const level = this._currentLevel;
            const center = (n - 1) / 2;

            for (let i = 0; i < n; i++) {
                const dist = Math.abs(i - center) / center;
                const envelope = Math.exp(-dist * dist * 3);
                const val = level * envelope;
                this._waveBars[i].scale_y = Math.max(MIN_SCALE, Math.min(MAX_SCALE, val));
            }
            return GLib.SOURCE_CONTINUE;
        });
    }

    _stopLevelWave() {
        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }
        this._currentLevel = 0;
    }

    // --- Visualization: upload animation ---

    _startUploadAnimation() {
        if (this._levelTimer) {
            GLib.Source.remove(this._levelTimer);
            this._levelTimer = null;
        }

        if (!this._waveBars) return;

        const startScales = this._waveBars.map(bar => bar.scale_y);
        const startColors = {
            center: { ...this._currentTheme.center },
            edge: { ...this._currentTheme.edge },
        };
        const targetColors = this._computeStateColors(State.UPLOADING);

        let time = 0;
        let transitionFrame = 0;
        const transitionFrames = 20;
        const n = this._waveBars.length;
        const center = (n - 1) / 2;

        this._uploadTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 50, () => {
            if (this._state !== State.UPLOADING || !this._waveBars) {
                this._uploadTimer = null;
                return GLib.SOURCE_REMOVE;
            }

            const t = Math.min(1, transitionFrame / transitionFrames);
            const ease = t * t * (3 - 2 * t);

            if (t < 1) {
                this._updateBarColors({
                    center: this._blendColor(startColors.center, targetColors.center, ease),
                    edge: this._blendColor(startColors.edge, targetColors.edge, ease),
                });
            } else if (transitionFrame === transitionFrames) {
                this._updateBarColors(targetColors);
            }

            for (let i = 0; i < n; i++) {
                const x = (i / (n - 1)) * Math.PI * 2;
                const w1 = Math.sin(x * 1.5) * Math.cos(time);
                const w2 = Math.sin(x * 2.5 + 0.5) * Math.sin(time * 0.7);
                const dist = Math.abs(i - center) / center;
                const envelope = 0.7 + 0.3 * Math.exp(-dist * dist * 2);
                const wave = w1 * 0.3 + w2 * 0.2;
                const dnaScale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, (0.45 + wave) * envelope));

                const scale = startScales[i] * (1 - ease) + dnaScale * ease;
                this._waveBars[i].scale_y = Math.max(MIN_SCALE, Math.min(MAX_SCALE, scale));
            }

            time += 0.1;
            if (transitionFrame < transitionFrames) transitionFrame++;
            return GLib.SOURCE_CONTINUE;
        });
    }

    // --- Visualization: finished ---

    _startFinishedAnimation() {
        this._cleanupAnimationTimers();

        if (!this._waveWidget || !this._waveBars) return;

        const startScales = this._waveBars.map(bar => bar.scale_y);
        let phase = 0;
        const totalFrames = 15;

        this._finishedTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 30, () => {
            if (!this._waveBars) {
                this._finishedTimer = null;
                return GLib.SOURCE_REMOVE;
            }

            phase++;
            const progress = phase / totalFrames;

            for (let i = 0; i < this._waveBars.length; i++) {
                this._waveBars[i].scale_y = Math.max(0.01, startScales[i] * (1 - progress));
            }

            if (phase >= totalFrames) {
                this._finishedTimer = null;
                this._state = State.IDLE;
                this._updateIndicator();
                this._hideWaveWidget();
                return GLib.SOURCE_REMOVE;
            }

            return GLib.SOURCE_CONTINUE;
        });
    }

    // --- D-Bus proxy ---

    _initDBusProxy() {
        this._dbusProxy = new VoicifyProxy(
            Gio.DBus.session,
            'com.dooshek.voicify',
            '/com/dooshek/voicify/Recorder'
        );

        this._dbusSignalIds.push(
            this._dbusProxy.connectSignal('RecordingStarted', () => {
                this._state = State.RECORDING;
                this._updateIndicator();
                this._showWaveWidget();
                this._startLevelWave();
            })
        );

        this._dbusSignalIds.push(
            this._dbusProxy.connectSignal('TranscriptionReady', (proxy, sender, [text]) => {
                this._onTranscriptionReady(text);
            })
        );

        this._dbusSignalIds.push(
            this._dbusProxy.connectSignal('PartialTranscription', (proxy, sender, [text]) => {
                this._onPartialTranscription(text);
            })
        );

        this._dbusSignalIds.push(
            this._dbusProxy.connectSignal('CompleteTranscription', (proxy, sender, [text]) => {
                this._onCompleteTranscription(text);
            })
        );

        this._dbusSignalIds.push(
            this._dbusProxy.connectSignal('RecordingError', (proxy, sender, [error]) => {
                console.error('Recording error:', error);
                this._state = State.IDLE;
                this._updateIndicator();
                this._stopLevelWave();
                this._hideWaveWidget();
            })
        );

        this._dbusSignalIds.push(
            this._dbusProxy.connectSignal('RecordingCancelled', () => {
                this._onRecordingCancelled();
            })
        );

        this._dbusSignalIds.push(
            this._dbusProxy.connectSignal('InputLevel', (proxy, sender, [level]) => {
                if (this._state !== State.RECORDING) return;
                if (typeof level === 'number' && isFinite(level)) {
                    this._pushLevel(level);
                }
            })
        );

        this._dbusSignalIds.push(
            this._dbusProxy.connectSignal('RequestPaste', (proxy, sender, [text]) => {
                this._onRequestPaste(text);
            })
        );
    }

    // --- D-Bus service file ---

    _ensureDBusServiceFile() {
        const serviceDir = GLib.get_home_dir() + '/.local/share/dbus-1/services';
        const serviceFile = serviceDir + '/' + DBUS_SERVICE_NAME + '.service';

        const file = Gio.File.new_for_path(serviceFile);
        if (file.query_exists(null)) return;

        const binaryFile = Gio.File.new_for_path(VOICIFY_BINARY_PATH);
        if (!binaryFile.query_exists(null)) {
            console.error('Voicify binary not found at:', VOICIFY_BINARY_PATH);
            return;
        }

        const dir = Gio.File.new_for_path(serviceDir);
        if (!dir.query_exists(null)) {
            try {
                dir.make_directory_with_parents(null);
            } catch (e) {
                console.error('Failed to create D-Bus services directory:', e.message);
                return;
            }
        }

        const serviceContent = `[D-BUS Service]
Name=${DBUS_SERVICE_NAME}
Exec=${VOICIFY_BINARY_PATH} --daemon --log-level=debug --log-filename=${VOICIFY_LOG_PATH}
`;

        try {
            const outputStream = file.replace(null, false, Gio.FileCreateFlags.NONE, null);
            const bytes = new TextEncoder().encode(serviceContent);
            outputStream.write_all(bytes, null);
            outputStream.close(null);

            Gio.DBus.session.call_sync(
                'org.freedesktop.DBus',
                '/org/freedesktop/DBus',
                'org.freedesktop.DBus',
                'ReloadConfig',
                null, null, Gio.DBusCallFlags.NONE, -1, null
            );
        } catch (e) {
            console.error('Failed to create D-Bus service file:', e.message);
        }
    }

    // --- Window detection ---

    _getFocusedWindow() {
        try {
            const focusedWindow = global.display.get_focus_window();
            if (!focusedWindow) return { title: '', app: '' };

            return {
                title: focusedWindow.get_title() || '',
                app: focusedWindow.get_wm_class() || '',
            };
        } catch (error) {
            console.error('Error getting focused window:', error);
            return { title: '', app: '' };
        }
    }

    _updateFocusedWindowInDaemon() {
        const { title, app } = this._getFocusedWindow();
        if (!this._dbusProxy) return;

        this._dbusProxy.UpdateFocusedWindowAsync(title, app)
            .catch(error => console.error('D-Bus: Failed to call UpdateFocusedWindow:', error));
    }
}
