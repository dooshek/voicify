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
import { loadDesigns, applyOverrides } from './designLoader.js';
import * as LayerPainter from './layerPainter.js';

// Size variants
const SIZES = {
    'thin':   { barWidth: 2, barSpacing: 2, numBars: 32, containerHeight: 40 },
    'medium': { barWidth: 4, barSpacing: 3, numBars: 26, containerHeight: 56 },
    'large':  { barWidth: 6, barSpacing: 4, numBars: 20, containerHeight: 72 },
};

// Terminal WM classes that use Ctrl+Shift+V instead of Ctrl+V for paste
const TERMINAL_WM_CLASSES = [
    'ghostty', 'gnome-terminal', 'gnome-terminal-server',
    'alacritty', 'kitty', 'wezterm', 'foot', 'tilix',
    'terminator', 'xfce4-terminal', 'konsole', 'yakuake',
    'guake', 'sakura', 'lxterminal', 'mate-terminal',
    'xterm', 'urxvt', 'st',
];

const MAX_SCALE = 1.0;
const MIN_SCALE = 0.04;

// Background padding for wave container (matches CSS padding in stylesheet.css)
const WAVE_H_PAD = 6;
const WAVE_V_PAD = 6;

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
    'phosphor':    { name: 'Phosphor',   center: {r: 50,  g: 255, b: 80},  edge: {r: 20,  g: 160, b: 40}  },
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
    <method name="SetAutoPausePlayback">
      <arg name="enabled" type="b" direction="in"/>
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
        this._pendingPaste = false;
        this._debounceMs = 500;
        this._virtualKeyboard = null;
        this._settingsChangedIds = [];
        this._currentTheme = THEMES['mint-dream'];
        this._designs = loadDesigns(this.path);
        this._currentDesign = this._designs.get('gnome') || this._designs.values().next().value;
        this._currentSize = SIZES['medium'];
        this._decorationWidgets = [];
        this._blurWidget = null;
        this._pixelGridWidget = null;
        this._waveContainer = null;
        this._modifier = 0;
        this._trailBars = null;
        this._trailLevel = 0;
        this._trailContainer = null;
        this._posX = 0.5;
        this._posY = 1.0;
        this._shadowPad = 0;
        this._widthPct = 100;
        this._heightPct = 100;
        this._isDragging = false;
        this._dragOffsetX = 0;
        this._dragOffsetY = 0;
        this._rebuildDebounceTimer = null;
        this._reactionTime = 10;
        this._smoothingAlpha = 0.5;
        this._sensitivityGain = 1.0;
        this._menuShortcutLabels = {};

        this._settings = this.getSettings();

        // Apply initial settings (isInitializing prevents design from resetting width/height)
        this._isInitializing = true;
        this._applyTheme();
        this._applyDesign();
        this._applySize();
        this._applyPosition();
        this._applyReactionTime();
        this._applySmoothing();
        this._applySensitivity();
        this._applyModifier();
        this._applyWidth();
        this._applyHeight();
        this._isInitializing = false;

        // Connect settings change handlers
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-theme', () => this._applyTheme())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-design', () => {
                this._applyDesign();
                // Reset width/height to design defaults when switching design
                const dc = this._currentDesign.container;
                this._settings.set_int('wave-width', dc.defaultWidth || 100);
                this._settings.set_int('wave-height', dc.defaultHeight || 100);
            })
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-size', () => this._applySize())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-type', () => this._applyWaveType())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-pos-x', () => this._applyPosition())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-pos-y', () => this._applyPosition())
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
            this._settings.connect('changed::wave-modifier', () => this._applyModifier())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-width', () => this._applyWidth())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::wave-height', () => this._applyHeight())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::design-overrides', () => this._applyDesign())
        );
        this._settingsChangedIds.push(
            this._settings.connect('changed::auto-pause-playback', () => this._syncAutoPausePlayback())
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
        this._syncAutoPausePlayback();
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

        this._destroyDecorations();
        this._destroyTrailBars();
        this._destroyBlur();
        this._destroyCanvasBackground();

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
        if (this._rebuildDebounceTimer) {
            GLib.Source.remove(this._rebuildDebounceTimer);
            this._rebuildDebounceTimer = null;
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

    _syncAutoPausePlayback() {
        if (!this._dbusProxy || !this._settings) return;
        const enabled = this._settings.get_boolean('auto-pause-playback');
        this._dbusProxy.SetAutoPausePlaybackAsync(enabled)
            .catch(e => console.debug('Voicify: SetAutoPausePlayback failed:', e.message));
    }

    _applyTheme() {
        const themeId = this._settings
            ? this._settings.get_string('wave-theme')
            : 'mint-dream';
        this._currentTheme = THEMES[themeId] || THEMES['mint-dream'];
        this._updateBarColors();
        this._updateTrailBarColors();
        this._updatePixelGridColors();

        // Repaint background (Cairo renders theme-aware border etc.)
        if (this._bgWidget) {
            this._bgWidget.queue_repaint();
        }
    }

    _applyWaveType() {
        // Wave type from settings overrides design defaults
        // Applies live: updates pivot and restarts animation if recording
        if (this._waveBars && this._state === State.RECORDING) {
            this._initFlatBars();
        }
    }

    _getEffectiveWaveType() {
        const settingType = this._settings
            ? this._settings.get_string('wave-type')
            : 'default';
        if (settingType !== 'default') return settingType;
        // Fallback to design defaults
        const db = this._currentDesign.bars;
        if (db.waveMode === 'wave') return 'wave';
        if (db.pivotY === 1.0) return 'bottom';
        if (db.pivotY === 0.0) return 'top';
        return 'center';
    }

    _getEffectiveNumBars() {
        const { numBars } = this._currentSize;
        return Math.round(numBars * (this._widthPct / 100));
    }

    _getEffectiveContainerHeight() {
        const { containerHeight } = this._currentSize;
        return Math.round(containerHeight * (this._heightPct / 100));
    }

    _applyDesign() {
        const designId = this._settings
            ? this._settings.get_string('wave-design')
            : 'gnome';
        const baseDesign = this._designs.get(designId) || this._designs.values().next().value;
        const overridesJson = this._settings
            ? this._settings.get_string('design-overrides')
            : '{}';
        this._currentDesign = applyOverrides(baseDesign, overridesJson, designId);

        // Rebuild widget if visible (preserve position)
        if (this._waveWidget && this._state === State.RECORDING) {
            this._rebuildWaveWidget();
        } else if (this._waveContainer) {
            this._updateBarColors();
        }
    }

    _applySize() {
        const sizeId = this._settings
            ? this._settings.get_string('wave-size')
            : 'medium';
        this._currentSize = SIZES[sizeId] || SIZES['medium'];

        this._rebuildWaveWidget();
    }

    _applyPosition() {
        if (this._settings) {
            this._posX = this._settings.get_double('wave-pos-x');
            this._posY = this._settings.get_double('wave-pos-y');
        }

        if (this._waveWidget && !this._isDragging) {
            const { barWidth, barSpacing } = this._currentSize;
            const numBars = this._getEffectiveNumBars();
            const containerHeight = this._getEffectiveContainerHeight();
            const effectiveWidth = barWidth + this._currentDesign.bars.widthAdjust;
            const containerWidth = numBars * (effectiveWidth + barSpacing) - barSpacing + WAVE_H_PAD * 2;
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

    _applyModifier() {
        const val = this._settings
            ? this._settings.get_int('wave-modifier')
            : 0;
        this._modifier = val / 100;

        // Rebuild widget to add/remove trail bars (preserve position)
        this._rebuildWaveWidget();
    }

    _applyWidth() {
        const val = this._settings
            ? this._settings.get_int('wave-width')
            : 100;
        this._widthPct = val;

        if (this._waveWidget && this._state === State.RECORDING) {
            this._debouncedRebuild();
        }
    }

    _applyHeight() {
        const val = this._settings
            ? this._settings.get_int('wave-height')
            : 100;
        this._heightPct = val;

        if (this._waveWidget && this._state === State.RECORDING) {
            this._debouncedRebuild();
        }
    }

    // --- Pixel Grid Widget - grid of small St.Widget pixels
    // width/height = totalWidth/totalHeight (matches shadow/bg area)
    _createPixelGridWidget(width, height) {
        const pixelGridLayer = (this._currentDesign.layers || []).find(l => l.type === 'pixelGrid');
        if (!pixelGridLayer) return;

        const dec = pixelGridLayer;
        const db = this._currentDesign.bars;
        const dc = this._currentDesign.container;
        const { barWidth } = this._currentSize;

        let center, edge;
        if (db.colorOverride) {
            const co = db.colorOverride;
            center = { r: co.center[0], g: co.center[1], b: co.center[2] };
            edge = { r: co.edge[0], g: co.edge[1], b: co.edge[2] };
        } else {
            center = this._currentTheme.center;
            edge = this._currentTheme.edge;
        }

        const alpha = dec.alpha || 0.12;
        const { barSpacing } = this._currentSize;
        const effectiveWidth = barWidth + db.widthAdjust;
        // Cell size = bar width, gap = bar spacing (uniform grid matching bars)
        const cellSize = dec.cellSize || effectiveWidth;
        const gap = dec.cellGap || barSpacing;
        const radius = Math.min(height / 2, dc.borderRadius);

        this._pixelGridWidget = new St.Widget({
            style: `border-radius: ${radius}px;`,
            clip_to_allocation: true,
            reactive: false,
            can_focus: false,
        });
        this._pixelGridWidget.set_size(width, height);

        const stride = cellSize + gap;
        // Fill entire area edge-to-edge (clip_to_allocation trims overflow)
        const numCols = Math.ceil(width / stride) + 1;
        const numRows = Math.ceil(height / stride) + 2;

        const midCol = (numCols - 1) / 2;

        for (let row = 0; row < numRows; row++) {
            for (let col = 0; col < numCols; col++) {
                const dist = midCol > 0 ? Math.abs(col - midCol) / midCol : 0;
                const colR = Math.round(center.r + (edge.r - center.r) * dist);
                const colG = Math.round(center.g + (edge.g - center.g) * dist);
                const colB = Math.round(center.b + (edge.b - center.b) * dist);

                const pixel = new St.Widget({
                    style: `background-color: rgba(${colR},${colG},${colB},${alpha});`,
                    reactive: false,
                    can_focus: false,
                    x: col * stride,
                    y: row * stride,
                    width: cellSize,
                    height: cellSize,
                });

                this._pixelGridWidget.add_child(pixel);
            }
        }

        // Widget is added as child by caller, not as separate chrome
    }

    _destroyPixelGrid() {
        if (this._pixelGridWidget) {
            this._pixelGridWidget.destroy();
            this._pixelGridWidget = null;
        }
    }

    _updatePixelGridColors() {
        if (!this._pixelGridWidget || !this._waveWidget || !this._bgWidget) return;
        const { barWidth, barSpacing } = this._currentSize;
        const numBars = this._getEffectiveNumBars();
        const containerHeight = this._getEffectiveContainerHeight();
        const db = this._currentDesign.bars;
        const effectiveWidth = barWidth + db.widthAdjust;
        const containerWidth = numBars * (effectiveWidth + barSpacing) - barSpacing;
        const totalWidth = containerWidth + WAVE_H_PAD * 2;
        const totalHeight = containerHeight + WAVE_V_PAD * 2;
        this._destroyPixelGrid();
        this._createPixelGridWidget(totalWidth, totalHeight);
        if (this._pixelGridWidget) {
            const sp = this._shadowPad || 0;
            this._pixelGridWidget.set_position(sp, sp);
            this._waveWidget.insert_child_above(this._pixelGridWidget, this._bgWidget);
        }
    }

    _updateBarColors(colors = null) {
        if (!this._waveBars) return;

        const db = this._currentDesign.bars;
        let center, edge;

        if (colors) {
            // Explicit colors passed (e.g. upload animation) - use as-is
            ({ center, edge } = colors);
        } else if (db.colorOverride) {
            const co = db.colorOverride;
            center = { r: co.center[0], g: co.center[1], b: co.center[2] };
            edge = { r: co.edge[0], g: co.edge[1], b: co.edge[2] };
        } else {
            ({ center, edge } = this._currentTheme);
        }

        const mute = db.colorMute || 0;
        if (mute > 0 && !colors) {
            const gray = { r: 160, g: 160, b: 160 };
            center = this._blendColor(center, gray, mute);
            edge = this._blendColor(edge, gray, mute);
        }

        const { barWidth, barSpacing } = this._currentSize;
        const effectiveWidth = barWidth + db.widthAdjust;
        const barCenter = (this._waveBars.length - 1) / 2;
        const containerHeight = this._getEffectiveContainerHeight();
        const barHeight = (containerHeight / 2) + 1;

        for (let i = 0; i < this._waveBars.length; i++) {
            const isLast = (i === this._waveBars.length - 1);
            const dist = Math.abs(i - barCenter) / barCenter;
            const r = Math.round(center.r + (edge.r - center.r) * dist);
            const g = Math.round(center.g + (edge.g - center.g) * dist);
            const b = Math.round(center.b + (edge.b - center.b) * dist);

            let style = `width: ${effectiveWidth}px; height: ${barHeight}px; background-color: rgb(${r},${g},${b}); border-radius: ${db.borderRadius}px;`;

            if (db.glowFromTheme) {
                style += ` box-shadow: 0 0 ${db.glowRadius}px rgba(${r},${g},${b},${db.glowAlpha});`;
            } else if (db.shadow) {
                style += ` box-shadow: ${db.shadow};`;
            }

            if (db.highlight) {
                style += ` border-top: 1px solid rgba(255,255,255,0.15);`;
            }

            if (!isLast) style += ` margin-right: ${barSpacing}px;`;
            this._waveBars[i].style = style;
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
            const shift = this._currentDesign.uploadHueShift ?? 120;
            return {
                center: this._rotateHue(center, shift),
                edge: this._rotateHue(edge, shift),
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
            this._state = State.FINISHED;
            this._isPostAutoPaste = false;
            this._updateIndicator();
            this._pendingPaste = true;
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

            const { app } = this._getFocusedWindow();
            const appLower = (app || '').toLowerCase();
            const isTerminal = TERMINAL_WM_CLASSES.some(t => appLower.includes(t));

            const eventTime = global.get_current_time();

            if (isTerminal) {
                // Ctrl+Shift+V for terminals
                this._virtualKeyboard.notify_keyval(eventTime, Clutter.KEY_Control_L, Clutter.KeyState.PRESSED);
                this._virtualKeyboard.notify_keyval(eventTime + 10, Clutter.KEY_Shift_L, Clutter.KeyState.PRESSED);
                this._virtualKeyboard.notify_keyval(eventTime + 20, Clutter.KEY_v, Clutter.KeyState.PRESSED);
                this._virtualKeyboard.notify_keyval(eventTime + 30, Clutter.KEY_v, Clutter.KeyState.RELEASED);
                this._virtualKeyboard.notify_keyval(eventTime + 40, Clutter.KEY_Shift_L, Clutter.KeyState.RELEASED);
                this._virtualKeyboard.notify_keyval(eventTime + 50, Clutter.KEY_Control_L, Clutter.KeyState.RELEASED);
            } else {
                // Ctrl+V for standard apps
                this._virtualKeyboard.notify_keyval(eventTime, Clutter.KEY_Control_L, Clutter.KeyState.PRESSED);
                this._virtualKeyboard.notify_keyval(eventTime + 10, Clutter.KEY_v, Clutter.KeyState.PRESSED);
                this._virtualKeyboard.notify_keyval(eventTime + 20, Clutter.KEY_v, Clutter.KeyState.RELEASED);
                this._virtualKeyboard.notify_keyval(eventTime + 30, Clutter.KEY_Control_L, Clutter.KeyState.RELEASED);
            }
        } catch (error) {
            console.debug('Virtual keyboard paste failed:', error.message);
        }
    }

    // --- Wave visualization widget ---

    _showWaveWidget() {
        if (this._waveWidget) return;

        const { barWidth, barSpacing } = this._currentSize;
        const numBars = this._getEffectiveNumBars();
        const containerHeight = this._getEffectiveContainerHeight();
        const db = this._currentDesign.bars;
        const dc = this._currentDesign.container;
        const effectiveWidth = barWidth + db.widthAdjust;
        const containerWidth = numBars * (effectiveWidth + barSpacing) - barSpacing;
        const barHeight = (containerHeight / 2) + 1;
        const totalWidth = containerWidth + WAVE_H_PAD * 2;
        const totalHeight = containerHeight + WAVE_V_PAD * 2;

        // Calculate shadow padding - DrawingArea needs extra space for Cairo shadow
        const shadowLayers = (this._currentDesign.layers || []).filter(l => l.type === 'shadow');
        let shadowPad = 0;
        for (const sl of shadowLayers) {
            const blur = sl.blur || 8;
            const ox = Math.abs(sl.x || 0);
            const oy = Math.abs(sl.y || 0);
            shadowPad = Math.max(shadowPad, blur + ox, blur + oy);
        }
        this._shadowPad = shadowPad;
        const outerWidth = totalWidth + shadowPad * 2;
        const outerHeight = totalHeight + shadowPad * 2;

        this._waveWidget = new St.Widget({
            style_class: 'voicify-wave-overlay',
            reactive: true,
            can_focus: false,
            track_hover: false,
            visible: true,
        });

        // Background: St.DrawingArea sized with shadow padding for Cairo shadow rendering
        this._bgWidget = new St.DrawingArea({ reactive: false });
        this._bgWidget.set_size(outerWidth, outerHeight);
        this._bgWidget.connect('repaint', (area) => {
            const cr = area.get_context();
            LayerPainter.drawAllCanvasLayersAt(cr, this._currentDesign, this._currentTheme,
                shadowPad, shadowPad, totalWidth, totalHeight);
            cr.$dispose();
        });
        this._waveWidget.add_child(this._bgWidget);

        // NOTE: Shell.BlurEffect is always rectangular - cannot be clipped to border-radius.
        // Blur disabled to avoid visible rectangular corners on rounded designs.

        // Pixel grid overlay (retro design) - offset into content area
        const hasPixelGrid = (this._currentDesign.layers || []).some(l => l.type === 'pixelGrid');
        if (hasPixelGrid) {
            this._createPixelGridWidget(totalWidth, totalHeight);
            if (this._pixelGridWidget) {
                this._pixelGridWidget.set_position(shadowPad, shadowPad);
                this._waveWidget.add_child(this._pixelGridWidget);
            }
        }

        this._waveContainer = new St.BoxLayout({
            style_class: 'voicify-wave-container',
            style: `border-radius: ${dc.borderRadius}px;`,
            vertical: false,
            x_align: Clutter.ActorAlign.CENTER,
            y_align: Clutter.ActorAlign.CENTER,
            x: shadowPad,
            y: shadowPad,
        });

        this._waveBars = [];

        let center, edge;
        if (db.colorOverride) {
            const co = db.colorOverride;
            center = { r: co.center[0], g: co.center[1], b: co.center[2] };
            edge = { r: co.edge[0], g: co.edge[1], b: co.edge[2] };
        } else {
            center = { ...this._currentTheme.center };
            edge = { ...this._currentTheme.edge };
        }

        const mute = db.colorMute || 0;
        if (mute > 0) {
            const gray = { r: 160, g: 160, b: 160 };
            center = this._blendColor(center, gray, mute);
            edge = this._blendColor(edge, gray, mute);
        }

        const barCenter = (numBars - 1) / 2;
        for (let i = 0; i < numBars; i++) {
            const isLast = (i === numBars - 1);
            const dist = Math.abs(i - barCenter) / barCenter;
            const r = Math.round(center.r + (edge.r - center.r) * dist);
            const g = Math.round(center.g + (edge.g - center.g) * dist);
            const b = Math.round(center.b + (edge.b - center.b) * dist);

            let barOpacity;
            if (db.opacityMode === 'uniform') {
                barOpacity = db.opacityUniform;
            } else {
                barOpacity = Math.round(db.opacityMin + (db.opacityMax - db.opacityMin) * (1 - dist));
            }

            let barStyle = `width: ${effectiveWidth}px; height: ${barHeight}px; background-color: rgb(${r},${g},${b}); border-radius: ${db.borderRadius}px;`;

            if (db.glowFromTheme) {
                barStyle += ` box-shadow: 0 0 ${db.glowRadius}px rgba(${r},${g},${b},${db.glowAlpha});`;
            } else if (db.shadow) {
                barStyle += ` box-shadow: ${db.shadow};`;
            }

            if (db.highlight) {
                barStyle += ` border-top: 1px solid rgba(255,255,255,0.15);`;
            }

            if (!isLast) barStyle += ` margin-right: ${barSpacing}px;`;

            const bar = new St.Widget({
                style_class: 'voicify-wave-bar',
                style: barStyle,
                visible: true,
                clip_to_allocation: true,
                opacity: barOpacity,
            });
            this._waveBars.push(bar);
            this._waveContainer.add_child(bar);
        }

        this._waveContainer.set_size(totalWidth, totalHeight);

        // Trail bars (shadow effect behind main bars, controlled by modifier)
        if (this._modifier > 0) {
            this._createTrailBars(numBars, effectiveWidth, barSpacing, barHeight, barCenter,
                center, edge, db, totalWidth, totalHeight);
            this._trailContainer.set_position(shadowPad, shadowPad);
            this._waveWidget.add_child(this._trailContainer);
        }

        this._waveWidget.add_child(this._waveContainer);

        try {
            this._createDecorations(totalWidth, totalHeight);
        } catch (e) {
            console.error(`Voicify: _createDecorations failed: ${e.message}\n${e.stack}`);
        }

        this._waveWidget.set_size(outerWidth, outerHeight);
        Main.layoutManager.addChrome(this._waveWidget);
        this._positionWaveWidget(totalWidth, totalHeight);

        // Drag-to-reposition
        this._waveWidget.connect('button-press-event', (actor, event) => {
            if (event.get_button() !== 1) return Clutter.EVENT_PROPAGATE;
            this._isDragging = true;
            const [px, py] = event.get_coords();
            this._dragOffsetX = px - actor.x;
            this._dragOffsetY = py - actor.y;
            return Clutter.EVENT_STOP;
        });

        this._waveWidget.connect('motion-event', (actor, event) => {
            if (!this._isDragging) return Clutter.EVENT_PROPAGATE;
            const [px, py] = event.get_coords();
            const newX = Math.round(px - this._dragOffsetX);
            const newY = Math.round(py - this._dragOffsetY);
            actor.set_position(newX, newY);
            return Clutter.EVENT_STOP;
        });

        this._waveWidget.connect('button-release-event', (actor, event) => {
            if (!this._isDragging) return Clutter.EVENT_PROPAGATE;
            this._saveWidgetPosition();
            console.debug(`Voicify: drag saved pos (${this._posX.toFixed(3)}, ${this._posY.toFixed(3)})`);
            this._isDragging = false;
            return Clutter.EVENT_STOP;
        });
    }

    _positionWaveWidget(containerWidth, containerHeight) {
        if (!this._waveWidget) return;
        const shadowPad = this._shadowPad || 0;

        const monitor = Main.layoutManager.primaryMonitor;

        // Usable area is full monitor (widget can be anywhere, including behind panel)
        const usableW = monitor.width - containerWidth;
        const usableH = monitor.height - containerHeight;

        // Content area position (0,0 = top-left of monitor, 1,1 = bottom-right)
        const contentX = monitor.x + Math.max(0, usableW) * this._posX;
        const contentY = monitor.y + Math.max(0, usableH) * this._posY;

        // waveWidget is larger by shadowPad on each side
        this._waveWidget.set_position(Math.round(contentX - shadowPad), Math.round(contentY - shadowPad));
        console.debug(`Voicify: position widget at pos (${this._posX.toFixed(3)}, ${this._posY.toFixed(3)}) -> pixel (${this._waveWidget.x}, ${this._waveWidget.y})`);
    }

    _saveWidgetPosition() {
        if (!this._waveWidget || !this._settings) return;
        const shadowPad = this._shadowPad || 0;

        const monitor = Main.layoutManager.primaryMonitor;
        // Content dimensions (without shadow padding)
        const containerWidth = this._waveWidget.width - shadowPad * 2;
        const containerHeight = this._waveWidget.height - shadowPad * 2;

        // Usable area is full monitor
        const usableW = monitor.width - containerWidth;
        const usableH = monitor.height - containerHeight;

        // Content position = waveWidget position + shadowPad
        const contentX = this._waveWidget.x + shadowPad;
        const contentY = this._waveWidget.y + shadowPad;
        const posX = usableW > 0 ? (contentX - monitor.x) / usableW : 0.5;
        const posY = usableH > 0 ? (contentY - monitor.y) / usableH : 0.5;

        this._posX = Math.max(0, Math.min(1, posX));
        this._posY = Math.max(0, Math.min(1, posY));
        this._settings.set_double('wave-pos-x', this._posX);
        this._settings.set_double('wave-pos-y', this._posY);
    }

    _preservePosition() {
        if (!this._waveWidget) return;
        const shadowPad = this._shadowPad || 0;
        const monitor = Main.layoutManager.primaryMonitor;
        const containerWidth = this._waveWidget.width - shadowPad * 2;
        const containerHeight = this._waveWidget.height - shadowPad * 2;
        // Usable area is full monitor
        const usableW = monitor.width - containerWidth;
        const usableH = monitor.height - containerHeight;
        const contentX = this._waveWidget.x + shadowPad;
        const contentY = this._waveWidget.y + shadowPad;
        this._posX = usableW > 0 ? Math.max(0, Math.min(1, (contentX - monitor.x) / usableW)) : 0.5;
        this._posY = usableH > 0 ? Math.max(0, Math.min(1, (contentY - monitor.y) / usableH)) : 0.5;
    }

    _rebuildWaveWidget() {
        if (!this._waveWidget || this._state !== State.RECORDING) return;
        this._preservePosition();
        this._hideWaveWidget();
        this._showWaveWidget();
        this._startLevelWave();
    }

    _debouncedRebuild() {
        if (this._rebuildDebounceTimer) {
            GLib.Source.remove(this._rebuildDebounceTimer);
            this._rebuildDebounceTimer = null;
        }
        // Update immediately just the in-memory values (already done by caller)
        // Delay the expensive widget rebuild
        this._rebuildDebounceTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 600, () => {
            this._rebuildDebounceTimer = null;
            this._rebuildWaveWidget();
            return GLib.SOURCE_REMOVE;
        });
    }

    _updateWaveWidget() {
        if (!this._waveWidget) return;

        this._waveWidget.style_class = 'voicify-wave-overlay uploading';
        this._stopLevelWave();
        if (this._trailContainer) {
            this._trailContainer.visible = false;
        }
        this._startUploadAnimation();
    }

    _hideWaveWidget() {
        // Always save position before destroying widget (safety net for missed button-release)
        if (this._waveWidget && this._settings) {
            this._preservePosition();
            this._settings.set_double('wave-pos-x', this._posX);
            this._settings.set_double('wave-pos-y', this._posY);
            console.debug(`Voicify: hide saved pos (${this._posX.toFixed(3)}, ${this._posY.toFixed(3)})`);
        }

        this._isDragging = false;
        this._cleanupAnimationTimers();
        this._destroyDecorations();
        this._destroyTrailBars();
        this._destroyPixelGrid();
        this._destroyBlur();

        if (this._waveWidget) {
            this._waveWidget.destroy();
            this._waveWidget = null;
            this._waveContainer = null;
            this._bgWidget = null;
            this._waveBars = null;
        }
    }

    _destroyCanvasBackground() {
        // bgWidget is a child of waveWidget, destroyed with it
        // but clear reference for safety
        this._bgWidget = null;
    }

    _destroyBlur() {
        if (this._blurWidget) {
            const effect = this._blurWidget.get_effect('blur');
            if (effect) this._blurWidget.remove_effect(effect);
            this._blurWidget.destroy();
            this._blurWidget = null;
        }
    }

    // --- Trail bars (shadow/echo effect) ---

    _createTrailBars(numBars, effectiveWidth, barSpacing, barHeight, barCenter,
                     center, edge, db, totalWidth, totalHeight) {
        this._trailContainer = new St.BoxLayout({
            style_class: 'voicify-wave-container',
            vertical: false,
            x_align: Clutter.ActorAlign.CENTER,
            y_align: Clutter.ActorAlign.CENTER,
        });

        this._trailBars = [];
        const trailOpacity = Math.round(this._modifier * 200);

        // Slightly dim colors for trail effect
        const darken = 0.7;
        const trailCenter = {
            r: Math.round(center.r * darken),
            g: Math.round(center.g * darken),
            b: Math.round(center.b * darken),
        };
        const trailEdge = {
            r: Math.round(edge.r * darken),
            g: Math.round(edge.g * darken),
            b: Math.round(edge.b * darken),
        };

        for (let i = 0; i < numBars; i++) {
            const isLast = (i === numBars - 1);
            const dist = Math.abs(i - barCenter) / barCenter;
            const r = Math.round(trailCenter.r + (trailEdge.r - trailCenter.r) * dist);
            const g = Math.round(trailCenter.g + (trailEdge.g - trailCenter.g) * dist);
            const b = Math.round(trailCenter.b + (trailEdge.b - trailCenter.b) * dist);

            let barStyle = `width: ${effectiveWidth}px; height: ${barHeight}px; background-color: rgb(${r},${g},${b}); border-radius: ${db.borderRadius}px;`;
            if (!isLast) barStyle += ` margin-right: ${barSpacing}px;`;

            const trailBar = new St.Widget({
                style_class: 'voicify-wave-bar',
                style: barStyle,
                visible: true,
                clip_to_allocation: true,
                opacity: trailOpacity,
            });
            trailBar.set_pivot_point(0.5, db.pivotY);
            trailBar.scale_y = db.scaleMin;
            this._trailBars.push(trailBar);
            this._trailContainer.add_child(trailBar);
        }

        this._trailContainer.set_size(totalWidth, totalHeight);
        this._trailLevel = 0;
    }

    _destroyTrailBars() {
        if (this._trailContainer) {
            this._trailContainer.destroy();
            this._trailContainer = null;
        }
        this._trailBars = null;
        this._trailLevel = 0;
    }

    _updateTrailBarColors() {
        if (!this._trailBars) return;

        const db = this._currentDesign.bars;
        let center, edge;

        if (db.colorOverride) {
            const co = db.colorOverride;
            center = { r: co.center[0], g: co.center[1], b: co.center[2] };
            edge = { r: co.edge[0], g: co.edge[1], b: co.edge[2] };
        } else {
            ({ center, edge } = this._currentTheme);
        }

        // Slightly dim colors for trail effect
        const darken = 0.7;
        center = { r: Math.round(center.r * darken), g: Math.round(center.g * darken), b: Math.round(center.b * darken) };
        edge = { r: Math.round(edge.r * darken), g: Math.round(edge.g * darken), b: Math.round(edge.b * darken) };

        const { barWidth, barSpacing } = this._currentSize;
        const effectiveWidth = barWidth + db.widthAdjust;
        const barCenter = (this._trailBars.length - 1) / 2;
        const containerHeight = this._getEffectiveContainerHeight();
        const barHeight = (containerHeight / 2) + 1;

        for (let i = 0; i < this._trailBars.length; i++) {
            const isLast = (i === this._trailBars.length - 1);
            const dist = Math.abs(i - barCenter) / barCenter;
            const r = Math.round(center.r + (edge.r - center.r) * dist);
            const g = Math.round(center.g + (edge.g - center.g) * dist);
            const b = Math.round(center.b + (edge.b - center.b) * dist);

            let style = `width: ${effectiveWidth}px; height: ${barHeight}px; background-color: rgb(${r},${g},${b}); border-radius: ${db.borderRadius}px;`;
            if (!isLast) style += ` margin-right: ${barSpacing}px;`;
            this._trailBars[i].style = style;
        }
    }

    // --- Decorations ---

    _createDecorations(containerWidth, containerHeight) {
        this._destroyDecorations();
        const layers = this._currentDesign.layers || [];

        // Only process widget layers (canvas layers are handled by layerPainter)
        const widgetTypes = ['scanlines', 'frame', 'highlightStrip', 'pixelGrid'];
        const widgetLayers = layers.filter(l => widgetTypes.includes(l.type));
        if (widgetLayers.length === 0) return;

        for (const layer of widgetLayers) {
            let widget = null;
            switch (layer.type) {
                case 'scanlines':
                    widget = this._createScanlines(layer, containerWidth, containerHeight);
                    break;
                case 'frame':
                    widget = this._createFrame(layer, containerWidth, containerHeight);
                    break;
                case 'highlightStrip':
                    widget = this._createHighlightStrip(layer, containerWidth, containerHeight);
                    break;
                case 'pixelGrid':
                    widget = this._createPixelGrid(layer, containerWidth, containerHeight);
                    break;
            }
            if (widget) {
                // Offset decoration into content area (past shadow padding)
                const sp = this._shadowPad || 0;
                widget.set_position(widget.x + sp, widget.y + sp);

                if (layer.position === 'background') {
                    // Insert behind bars (before trail/main containers)
                    if (this._trailContainer) {
                        this._waveWidget.insert_child_below(widget, this._trailContainer);
                    } else {
                        this._waveWidget.insert_child_below(widget, this._waveContainer);
                    }
                } else {
                    this._waveWidget.add_child(widget);
                }
                this._decorationWidgets.push(widget);
            }
        }
    }

    _destroyDecorations() {
        if (this._decorationWidgets) {
            for (const w of this._decorationWidgets) {
                w?.destroy();
            }
        }
        this._decorationWidgets = [];
    }

    _createPixelGrid(dec, containerWidth, containerHeight) {
        // pixelGrid is now created as separate chrome in _createPixelGridWidget (before blur)
        // This method is called by _createDecorations but we return null as the widget is already created
        return null;
    }

    _createScanlines(dec, containerWidth, containerHeight) {
        const [cr, cg, cb] = dec.color || [0, 0, 0];
        const alpha = Math.round((dec.alpha || 0.15) * 255);
        const lineHeight = dec.lineHeight || 1;
        const lineSpacing = dec.lineSpacing || 3;
        const radius = dec.borderRadius || 0;

        const box = new St.BoxLayout({
            vertical: true,
            style: `border-radius: ${radius}px;`,
            clip_to_allocation: true,
            x: 0,
            y: 0,
        });
        box.set_size(containerWidth, containerHeight);

        const totalLines = Math.floor(containerHeight / (lineHeight + lineSpacing));
        for (let i = 0; i < totalLines; i++) {
            const line = new St.Widget({
                style: `background-color: rgb(${cr},${cg},${cb}); height: ${lineHeight}px; margin-bottom: ${lineSpacing}px;`,
                opacity: alpha,
            });
            box.add_child(line);
        }

        return box;
    }

    _createFrame(dec, containerWidth, containerHeight) {
        const [cr, cg, cb] = dec.color || [128, 128, 128];
        const alpha = dec.alpha || 0.7;
        const inset = dec.inset || 0;
        const bw = dec.borderWidth || 2;
        const radius = dec.borderRadius || 20;

        const x = inset;
        const y = inset;
        const w = containerWidth - 2 * inset;
        const h = containerHeight - 2 * inset;

        let style = `border: ${bw}px solid rgba(${cr},${cg},${cb},${alpha}); border-radius: ${radius}px; background-color: transparent;`;
        if (dec.shadow) {
            style += ` box-shadow: ${dec.shadow};`;
        }

        const frame = new St.Widget({
            style: style,
            x: x,
            y: y,
        });
        frame.set_size(w, h);

        return frame;
    }

    _createHighlightStrip(dec, containerWidth, containerHeight) {
        const [cr, cg, cb] = dec.color || [255, 255, 255];
        const alpha = dec.alpha || 0.08;
        const height = dec.height || 2;
        const topOffset = dec.topOffset || 5;
        const radius = dec.borderRadius || 20;
        const hPad = 10;

        const strip = new St.Widget({
            style: `background-color: rgba(${cr},${cg},${cb},${alpha}); border-radius: ${radius}px;`,
            x: hPad,
            y: topOffset,
        });
        strip.set_size(containerWidth - hPad * 2, height);

        return strip;
    }

    // --- Visualization: recording level ---

    _initFlatBars() {
        if (!this._waveBars) return;
        this._currentLevel = 0;
        this._trailLevel = 0;
        this._wavePhase = 0;
        const waveType = this._getEffectiveWaveType();
        const pivotY = waveType === 'bottom' ? 1.0
                     : waveType === 'top' ? 0.0
                     : 0.5;
        const scaleMin = this._currentDesign.bars.scaleMin;
        this._waveBars.forEach(bar => {
            bar.set_pivot_point(0.5, pivotY);
            bar.scale_y = scaleMin;
        });
        if (this._trailBars) {
            this._trailBars.forEach(bar => {
                bar.set_pivot_point(0.5, pivotY);
                bar.scale_y = scaleMin;
            });
        }
    }

    _pushLevel(level) {
        if (!this._waveBars) return;
        const db = this._currentDesign.bars;
        const amplified = Math.min(db.scaleMax, level * this._sensitivityGain);
        const raw = Math.max(db.scaleMin, amplified);
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
            const db = this._currentDesign.bars;
            const waveType = this._getEffectiveWaveType();

            if (waveType === 'wave') {
                // Propagating wave: per-bar phase offset, left-to-right
                const waveSpeed = db.waveSpeed || 0.15;
                this._wavePhase += waveSpeed;
                for (let i = 0; i < n; i++) {
                    const barPhase = this._wavePhase + (i / (n - 1)) * Math.PI * 2;
                    const waveValue = 0.5 + 0.5 * Math.sin(barPhase);
                    const val = level * waveValue;
                    this._waveBars[i].scale_y = Math.max(db.scaleMin, Math.min(db.scaleMax, val));
                }
            } else {
                // Default "level" mode: Gaussian envelope from center
                for (let i = 0; i < n; i++) {
                    const dist = Math.abs(i - center) / center;
                    const envelope = Math.exp(-dist * dist * 3);
                    const val = level * envelope;
                    this._waveBars[i].scale_y = Math.max(db.scaleMin, Math.min(db.scaleMax, val));
                }
            }

            // Update trail bars (peak-hold with slow decay)
            if (this._trailBars && this._modifier > 0) {
                const trailDecay = 0.93 + this._modifier * 0.06;
                if (level > this._trailLevel) {
                    this._trailLevel = 0.4 * level + 0.6 * this._trailLevel;
                } else {
                    this._trailLevel = this._trailLevel * trailDecay;
                }

                for (let i = 0; i < n; i++) {
                    if (waveType === 'wave') {
                        const barPhase = this._wavePhase + (i / (n - 1)) * Math.PI * 2;
                        const waveValue = 0.5 + 0.5 * Math.sin(barPhase);
                        const val = this._trailLevel * waveValue;
                        this._trailBars[i].scale_y = Math.max(db.scaleMin, Math.min(db.scaleMax, val));
                    } else {
                        const dist = Math.abs(i - center) / center;
                        const envelope = Math.exp(-dist * dist * 3);
                        const val = this._trailLevel * envelope;
                        this._trailBars[i].scale_y = Math.max(db.scaleMin, Math.min(db.scaleMax, val));
                    }
                }
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
        this._trailLevel = 0;
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

        const db = this._currentDesign.bars;

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
                const dnaScale = Math.max(db.scaleMin, Math.min(db.scaleMax, (0.45 + wave) * envelope));

                const scale = startScales[i] * (1 - ease) + dnaScale * ease;
                this._waveBars[i].scale_y = Math.max(db.scaleMin, Math.min(db.scaleMax, scale));
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
        const barFrames = 17;
        const fadeFrames = 8;
        const totalFrames = barFrames + fadeFrames;

        this._finishedTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 30, () => {
            if (!this._waveWidget) {
                this._finishedTimer = null;
                return GLib.SOURCE_REMOVE;
            }

            phase++;

            if (phase <= barFrames) {
                // Phase 1: bars scale down
                if (this._waveBars) {
                    const progress = phase / barFrames;
                    for (let i = 0; i < this._waveBars.length; i++) {
                        this._waveBars[i].scale_y = Math.max(0.01, startScales[i] * (1 - progress));
                    }
                }
            } else {
                // Phase 2: whole box fades out with opacity (blur is child, inherits opacity)
                const fadeProgress = (phase - barFrames) / fadeFrames;
                this._waveWidget.opacity = Math.round(255 * (1 - fadeProgress));
            }

            if (phase >= totalFrames) {
                this._finishedTimer = null;
                // Paste before hiding (timer cleanup won't cancel it)
                if (this._pendingPaste) {
                    this._pendingPaste = false;
                    this._performAutoPaste();
                }
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
