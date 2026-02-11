'use strict';

import Adw from 'gi://Adw';
import Gtk from 'gi://Gtk';
import Gdk from 'gi://Gdk';
import GLib from 'gi://GLib';
import Gio from 'gi://Gio';

import { ExtensionPreferences } from 'resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js';
import { loadDesigns, getDesignIds } from './designLoader.js';
// Color themes (duplicated from extension.js - prefs.js runs in a different process)
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

const THEME_IDS = Object.keys(THEMES);
const SIZE_IDS = ['thin', 'medium', 'large'];

// Modifier mask for accelerator capture
const ACCEL_MODS = Gdk.ModifierType.CONTROL_MASK |
                   Gdk.ModifierType.SHIFT_MASK |
                   Gdk.ModifierType.ALT_MASK |
                   Gdk.ModifierType.SUPER_MASK;

export default class VoicifyPreferences extends ExtensionPreferences {
    fillPreferencesWindow(window) {
        window.set_default_size(500, 700);

        const settings = this.getSettings();
        const settingsHandlerIds = [];

        const designs = loadDesigns(this.path);
        const DESIGN_IDS = getDesignIds(designs);

        const page = new Adw.PreferencesPage({
            title: 'Appearance',
            icon_name: 'preferences-desktop-appearance-symbolic',
        });
        window.add(page);

        // --- Style group ---
        const styleGroup = new Adw.PreferencesGroup({
            title: 'Theme',
            description: 'Use keyboard shortcuts to start recording and see the visualization live',
        });
        page.add(styleGroup);

        // Design style combo (first)
        const designModel = new Gtk.StringList();
        for (const id of DESIGN_IDS) {
            designModel.append(designs.get(id).name);
        }

        const designRow = new Adw.ComboRow({
            title: 'Style',
            subtitle: 'Visual rendering style for the waveform',
            model: designModel,
        });

        const currentDesignId = settings.get_string('wave-design');
        const currentDesignIdx = DESIGN_IDS.indexOf(currentDesignId);
        if (currentDesignIdx >= 0) designRow.set_selected(currentDesignIdx);

        designRow.connect('notify::selected', () => {
            const idx = designRow.get_selected();
            if (idx >= 0 && idx < DESIGN_IDS.length) {
                settings.set_string('wave-design', DESIGN_IDS[idx]);
            }
        });
        styleGroup.add(designRow);

        // Color theme combo (second)
        const themeModel = new Gtk.StringList();
        for (const id of THEME_IDS) {
            themeModel.append(THEMES[id].name);
        }

        const themeRow = new Adw.ComboRow({
            title: 'Color',
            model: themeModel,
        });

        const currentThemeId = settings.get_string('wave-theme');
        const currentThemeIdx = THEME_IDS.indexOf(currentThemeId);
        if (currentThemeIdx >= 0) themeRow.set_selected(currentThemeIdx);

        themeRow.connect('notify::selected', () => {
            const idx = themeRow.get_selected();
            if (idx >= 0 && idx < THEME_IDS.length) {
                settings.set_string('wave-theme', THEME_IDS[idx]);
            }
        });
        styleGroup.add(themeRow);

        // Wave Type (moved from Size to Style)
        const WAVE_TYPES = ['default', 'bottom', 'center', 'top', 'wave'];
        const WAVE_TYPE_LABELS = ['Default', 'Rise', 'Pulse', 'Drop', 'Wave'];

        const waveTypeModel = new Gtk.StringList();
        for (const label of WAVE_TYPE_LABELS) {
            waveTypeModel.append(label);
        }

        const waveTypeRow = new Adw.ComboRow({
            title: 'Wave Type',
            subtitle: 'Animation direction for the bars',
            model: waveTypeModel,
        });

        const currentWaveType = settings.get_string('wave-type');
        const currentWaveTypeIdx = WAVE_TYPES.indexOf(currentWaveType);
        if (currentWaveTypeIdx >= 0) waveTypeRow.set_selected(currentWaveTypeIdx);

        waveTypeRow.connect('notify::selected', () => {
            const idx = waveTypeRow.get_selected();
            if (idx >= 0 && idx < WAVE_TYPES.length) {
                settings.set_string('wave-type', WAVE_TYPES[idx]);
            }
        });
        styleGroup.add(waveTypeRow);

        // Shadow Trail slider
        const trailRow = new Adw.ActionRow({
            title: 'Shadow Trail',
            subtitle: 'Echo effect behind the waveform bars',
        });

        const trailAdj = new Gtk.Adjustment({
            lower: 0,
            upper: 100,
            step_increment: 1,
            page_increment: 10,
            value: settings.get_int('wave-modifier'),
        });

        const trailScale = new Gtk.Scale({
            adjustment: trailAdj,
            orientation: Gtk.Orientation.HORIZONTAL,
            draw_value: false,
            hexpand: true,
            valign: Gtk.Align.CENTER,
            width_request: 200,
        });

        trailScale.connect('value-changed', () => {
            settings.set_int('wave-modifier', Math.round(trailScale.get_value()));
        });

        trailRow.add_suffix(trailScale);
        styleGroup.add(trailRow);

        // --- Size group ---
        const sizeGroup = new Adw.PreferencesGroup({
            title: 'Size',
        });
        page.add(sizeGroup);

        const sizeModel = new Gtk.StringList();
        sizeModel.append('Thin');
        sizeModel.append('Medium');
        sizeModel.append('Large');

        const sizeRow = new Adw.ComboRow({
            title: 'Bar Size',
            model: sizeModel,
        });

        const currentSizeId = settings.get_string('wave-size');
        const currentSizeIdx = SIZE_IDS.indexOf(currentSizeId);
        if (currentSizeIdx >= 0) sizeRow.set_selected(currentSizeIdx);

        sizeRow.connect('notify::selected', () => {
            const idx = sizeRow.get_selected();
            if (idx >= 0 && idx < SIZE_IDS.length) {
                settings.set_string('wave-size', SIZE_IDS[idx]);
            }
        });
        sizeGroup.add(sizeRow);

        // Width slider
        const widthRow = new Adw.ActionRow({
            title: 'Width',
            subtitle: 'Widget width (50–500%)',
        });

        const widthAdj = new Gtk.Adjustment({
            lower: 50,
            upper: 500,
            step_increment: 5,
            page_increment: 25,
            value: settings.get_int('wave-width'),
        });

        const widthScale = new Gtk.Scale({
            adjustment: widthAdj,
            orientation: Gtk.Orientation.HORIZONTAL,
            draw_value: true,
            hexpand: true,
            valign: Gtk.Align.CENTER,
            width_request: 200,
        });
        widthScale.set_format_value_func((scale, value) => `${Math.round(value)}%`);

        widthScale.connect('value-changed', () => {
            settings.set_int('wave-width', Math.round(widthScale.get_value()));
        });

        widthRow.add_suffix(widthScale);
        sizeGroup.add(widthRow);

        // Height slider
        const heightRow = new Adw.ActionRow({
            title: 'Height',
            subtitle: 'Widget height (50–500%)',
        });

        const heightAdj = new Gtk.Adjustment({
            lower: 50,
            upper: 500,
            step_increment: 5,
            page_increment: 25,
            value: settings.get_int('wave-height'),
        });

        const heightScale = new Gtk.Scale({
            adjustment: heightAdj,
            orientation: Gtk.Orientation.HORIZONTAL,
            draw_value: true,
            hexpand: true,
            valign: Gtk.Align.CENTER,
            width_request: 200,
        });
        heightScale.set_format_value_func((scale, value) => `${Math.round(value)}%`);

        heightScale.connect('value-changed', () => {
            settings.set_int('wave-height', Math.round(heightScale.get_value()));
        });

        heightRow.add_suffix(heightScale);
        sizeGroup.add(heightRow);

        // --- Responsiveness group ---
        const responsivenessGroup = new Adw.PreferencesGroup({
            title: 'Responsiveness',
        });
        page.add(responsivenessGroup);

        const responsivenessRow = new Adw.ActionRow({
            title: 'Responsiveness',
            subtitle: 'How quickly the waveform reacts',
        });

        const respAdj = new Gtk.Adjustment({
            lower: 5,
            upper: 40,
            step_increment: 1,
            page_increment: 5,
            value: settings.get_int('reaction-time'),
        });

        const respScale = new Gtk.Scale({
            adjustment: respAdj,
            orientation: Gtk.Orientation.HORIZONTAL,
            draw_value: false,
            hexpand: true,
            inverted: true,
            valign: Gtk.Align.CENTER,
            width_request: 200,
        });

        respScale.connect('value-changed', () => {
            settings.set_int('reaction-time', Math.round(respScale.get_value()));
        });

        responsivenessRow.add_suffix(respScale);
        responsivenessGroup.add(responsivenessRow);

        // Smoothing slider
        const smoothingRow = new Adw.ActionRow({
            title: 'Smoothing',
            subtitle: 'How fluid the bar animations are',
        });

        const smoothAdj = new Gtk.Adjustment({
            lower: 0,
            upper: 100,
            step_increment: 1,
            page_increment: 10,
            value: settings.get_int('wave-smoothing'),
        });

        const smoothScale = new Gtk.Scale({
            adjustment: smoothAdj,
            orientation: Gtk.Orientation.HORIZONTAL,
            draw_value: false,
            hexpand: true,
            valign: Gtk.Align.CENTER,
            width_request: 200,
        });

        smoothScale.connect('value-changed', () => {
            settings.set_int('wave-smoothing', Math.round(smoothScale.get_value()));
        });

        smoothingRow.add_suffix(smoothScale);
        responsivenessGroup.add(smoothingRow);

        // Sensitivity slider
        const sensitivityRow = new Adw.ActionRow({
            title: 'Sensitivity',
            subtitle: 'How much the bars react to sound',
        });

        const sensAdj = new Gtk.Adjustment({
            lower: 50,
            upper: 300,
            step_increment: 5,
            page_increment: 25,
            value: settings.get_int('wave-sensitivity'),
        });

        const sensScale = new Gtk.Scale({
            adjustment: sensAdj,
            orientation: Gtk.Orientation.HORIZONTAL,
            draw_value: false,
            hexpand: true,
            valign: Gtk.Align.CENTER,
            width_request: 200,
        });

        sensScale.connect('value-changed', () => {
            settings.set_int('wave-sensitivity', Math.round(sensScale.get_value()));
        });

        sensitivityRow.add_suffix(sensScale);
        responsivenessGroup.add(sensitivityRow);

        // === Shortcuts page (second tab) ===
        const shortcutsPage = new Adw.PreferencesPage({
            title: 'Shortcuts',
            icon_name: 'preferences-desktop-keyboard-shortcuts-symbolic',
        });
        window.add(shortcutsPage);

        const shortcutsGroup = new Adw.PreferencesGroup();
        shortcutsPage.add(shortcutsGroup);

        _addShortcutRow(shortcutsGroup, 'Realtime', 'shortcut-realtime', settings, window);
        _addShortcutRow(shortcutsGroup, 'Post + auto-paste', 'shortcut-post-autopaste', settings, window);
        _addShortcutRow(shortcutsGroup, 'Post + router', 'shortcut-post-router', settings, window);
        _addShortcutRow(shortcutsGroup, 'Cancel', 'shortcut-cancel', settings, window);

        // === Transcription page (third tab) ===
        const transcriptionPage = new Adw.PreferencesPage({
            title: 'Transcription',
            icon_name: 'audio-input-microphone-symbolic',
        });
        window.add(transcriptionPage);

        // --- Standard transcription group ---
        const standardGroup = new Adw.PreferencesGroup({
            title: 'Post-processing',
            description: 'Model for Ctrl+Super+C and Ctrl+Super+D modes',
        });
        transcriptionPage.add(standardGroup);

        const STANDARD_MODELS = [
            { id: 'gpt-4o-mini-transcribe', name: 'GPT-4o Mini Transcribe', desc: 'Fast, cost-effective ($0.003/min)' },
            { id: 'gpt-4o-transcribe', name: 'GPT-4o Transcribe', desc: 'Most accurate ($0.006/min)' },
        ];

        const standardModel = new Gtk.StringList();
        for (const m of STANDARD_MODELS) {
            standardModel.append(m.name);
        }

        const standardRow = new Adw.ComboRow({
            title: 'Model',
            subtitle: STANDARD_MODELS.find(m => m.id === settings.get_string('transcription-model'))?.desc || '',
            model: standardModel,
        });

        const curStandardIdx = STANDARD_MODELS.findIndex(m => m.id === settings.get_string('transcription-model'));
        if (curStandardIdx >= 0) standardRow.set_selected(curStandardIdx);

        standardRow.connect('notify::selected', () => {
            const idx = standardRow.get_selected();
            if (idx >= 0 && idx < STANDARD_MODELS.length) {
                settings.set_string('transcription-model', STANDARD_MODELS[idx].id);
                standardRow.subtitle = STANDARD_MODELS[idx].desc;
            }
        });
        standardGroup.add(standardRow);

        // --- Realtime transcription group ---
        const realtimeGroup = new Adw.PreferencesGroup({
            title: 'Realtime',
            description: 'Model for Ctrl+Super+V streaming mode',
        });
        transcriptionPage.add(realtimeGroup);

        const REALTIME_MODELS = [
            { id: 'gpt-4o-mini-transcribe', name: 'GPT-4o Mini Transcribe', desc: 'Fast, cost-effective ($0.003/min)' },
            { id: 'gpt-4o-transcribe', name: 'GPT-4o Transcribe', desc: 'Most accurate ($0.006/min)' },
        ];

        const realtimeModelList = new Gtk.StringList();
        for (const m of REALTIME_MODELS) {
            realtimeModelList.append(m.name);
        }

        const realtimeRow = new Adw.ComboRow({
            title: 'Model',
            subtitle: REALTIME_MODELS.find(m => m.id === settings.get_string('realtime-model'))?.desc || '',
            model: realtimeModelList,
        });

        const curRealtimeIdx = REALTIME_MODELS.findIndex(m => m.id === settings.get_string('realtime-model'));
        if (curRealtimeIdx >= 0) realtimeRow.set_selected(curRealtimeIdx);

        realtimeRow.connect('notify::selected', () => {
            const idx = realtimeRow.get_selected();
            if (idx >= 0 && idx < REALTIME_MODELS.length) {
                settings.set_string('realtime-model', REALTIME_MODELS[idx].id);
                realtimeRow.subtitle = REALTIME_MODELS[idx].desc;
            }
        });
        realtimeGroup.add(realtimeRow);

        // Info row
        const infoGroup = new Adw.PreferencesGroup();
        transcriptionPage.add(infoGroup);

        const infoRow = new Adw.ActionRow({
            title: 'Pricing',
            subtitle: 'GPT-4o Mini: $0.003/min \u00b7 GPT-4o: $0.006/min\nBoth models support Polish and 50+ languages',
        });
        infoGroup.add(infoRow);

        // === Advanced page (fourth tab) ===
        const advancedPage = new Adw.PreferencesPage({
            title: 'Advanced',
            icon_name: 'preferences-other-symbolic',
        });
        window.add(advancedPage);

        // --- Behavior group ---
        const behaviorGroup = new Adw.PreferencesGroup({
            title: 'Behavior',
        });
        advancedPage.add(behaviorGroup);

        const autoPauseRow = new Adw.SwitchRow({
            title: 'Auto-pause Playback',
            subtitle: 'Pause media during recording, resume after',
        });
        settings.bind('auto-pause-playback', autoPauseRow, 'active', Gio.SettingsBindFlags.DEFAULT);
        behaviorGroup.add(autoPauseRow);

        // Check playerctl when auto-pause is toggled ON
        autoPauseRow.connect('notify::active', () => {
            if (!autoPauseRow.active) return;
            if (GLib.find_program_in_path('playerctl')) return;

            // Block - revert immediately
            settings.set_boolean('auto-pause-playback', false);

            const cmd = _getInstallCommand();
            const dialog = new Adw.AlertDialog({
                heading: 'playerctl is not installed',
                body: 'Auto-pause requires playerctl to reliably control media playback.\n\nInstall it first, then enable this option.\n\nRun in terminal:',
                content_width: 460,
            });

            const cmdEntry = new Gtk.Entry({
                text: cmd,
                editable: false,
                hexpand: true,
            });
            cmdEntry.add_css_class('monospace');
            dialog.set_extra_child(cmdEntry);

            dialog.add_response('ok', 'OK');
            dialog.set_default_response('ok');

            dialog.present(window);
        });

        // Enter after paste delay
        const enterDelayRow = new Adw.ActionRow({
            title: 'Send After Paste Delay',
            subtitle: 'Wait time before sending Enter',
        });
        const enterDelayAdj = new Gtk.Adjustment({
            lower: 100, upper: 5000, step_increment: 100, page_increment: 500,
            value: settings.get_int('enter-after-paste-delay'),
        });
        const enterDelayScale = new Gtk.Scale({
            adjustment: enterDelayAdj, draw_value: true, digits: 0,
            width_request: 200, valign: Gtk.Align.CENTER,
        });
        enterDelayScale.set_format_value_func((_scale, value) => `${Math.round(value)} ms`);
        enterDelayScale.connect('value-changed', () => {
            settings.set_int('enter-after-paste-delay', Math.round(enterDelayAdj.get_value()));
        });
        enterDelayRow.add_suffix(enterDelayScale);
        behaviorGroup.add(enterDelayRow);

        // --- Design overrides group ---
        const designOverridesGroup = new Adw.PreferencesGroup({
            title: 'Design Overrides',
            description: 'Fine-tune parameters of the current style',
        });
        advancedPage.add(designOverridesGroup);

        const advancedRow = new Adw.SwitchRow({
            title: 'Enable Overrides',
            subtitle: 'Show per-design parameter controls below',
        });
        settings.bind('advanced-settings', advancedRow, 'active', Gio.SettingsBindFlags.DEFAULT);
        designOverridesGroup.add(advancedRow);

        // Advanced settings panel state
        let advancedGroups = [];

        const destroyAdvancedGroups = () => {
            for (const g of advancedGroups) {
                advancedPage.remove(g);
            }
            advancedGroups = [];
        };

        const buildAdvancedGroups = () => {
            destroyAdvancedGroups();
            const designId = settings.get_string('wave-design');
            const design = designs.get(designId);
            if (!design) return;

            const groups = _buildAdvancedGroups(settings, designs, designId);
            for (const g of groups) {
                advancedPage.add(g);
            }
            advancedGroups = groups;
        };

        // Initial build if already enabled
        if (settings.get_boolean('advanced-settings')) {
            buildAdvancedGroups();
        }

        // Toggle Advanced ON/OFF
        settingsHandlerIds.push(
            settings.connect('changed::advanced-settings', () => {
                if (settings.get_boolean('advanced-settings')) {
                    buildAdvancedGroups();
                } else {
                    destroyAdvancedGroups();
                }
            })
        );

        // Rebuild when design changes + advanced is ON
        settingsHandlerIds.push(
            settings.connect('changed::wave-design', () => {
                if (settings.get_boolean('advanced-settings')) {
                    buildAdvancedGroups();
                }
            })
        );

        // External settings change listeners
        settingsHandlerIds.push(
            settings.connect('changed::wave-theme', () => {
                const idx = THEME_IDS.indexOf(settings.get_string('wave-theme'));
                if (idx >= 0) themeRow.set_selected(idx);
            })
        );

        settingsHandlerIds.push(
            settings.connect('changed::wave-design', () => {
                const idx = DESIGN_IDS.indexOf(settings.get_string('wave-design'));
                if (idx >= 0) designRow.set_selected(idx);
                // Update width/height sliders from design defaults
                const designId = settings.get_string('wave-design');
                const design = designs.get(designId);
                if (design) {
                    widthAdj.set_value(design.container.defaultWidth || 100);
                    heightAdj.set_value(design.container.defaultHeight || 100);
                }
            })
        );

        settingsHandlerIds.push(
            settings.connect('changed::wave-modifier', () => {
                trailAdj.set_value(settings.get_int('wave-modifier'));
            })
        );

        settingsHandlerIds.push(
            settings.connect('changed::wave-width', () => {
                widthAdj.set_value(settings.get_int('wave-width'));
            })
        );

        settingsHandlerIds.push(
            settings.connect('changed::wave-height', () => {
                heightAdj.set_value(settings.get_int('wave-height'));
            })
        );

        // === Stats page (fifth tab) ===
        const statsPage = new Adw.PreferencesPage({
            title: 'Stats',
            icon_name: 'utilities-system-monitor-symbolic',
        });
        window.add(statsPage);

        const statsGroup = new Adw.PreferencesGroup({
            title: 'Recording Statistics',
            description: 'Time spent recording per transcription model',
        });
        statsPage.add(statsGroup);

        const statsLoadingRow = new Adw.ActionRow({
            title: 'Loading...',
            subtitle: 'Connecting to voicify daemon',
        });
        statsGroup.add(statsLoadingRow);

        // D-Bus proxy for stats
        const VoicifyStatsInterface = `
<node>
  <interface name="com.dooshek.voicify.Recorder">
    <method name="GetRecordingStats">
      <arg name="stats_json" type="s" direction="out"/>
    </method>
    <method name="ResetRecordingStats"/>
  </interface>
</node>`;
        const StatsProxy = Gio.DBusProxy.makeProxyWrapper(VoicifyStatsInterface);

        let statsProxy = null;
        let statsRows = [];

        const clearStatsRows = () => {
            for (const r of statsRows) {
                statsGroup.remove(r);
            }
            statsRows = [];
        };

        // OpenAI transcription pricing (USD per minute)
        const MODEL_PRICING = {
            'whisper-1': 0.006,
            'gpt-4o-transcribe': 0.006,
            'gpt-4o-mini-transcribe': 0.003,
            // Groq models - free tier / very cheap
            'whisper-large-v3': 0,
            'whisper-large-v3-turbo': 0,
            'distil-whisper-large-v3-en': 0,
        };

        const formatDuration = (totalSeconds) => {
            const mins = Math.floor(totalSeconds / 60);
            const secs = Math.round(totalSeconds % 60);
            if (mins === 0) return `${secs}s`;
            return `${mins}m ${secs}s`;
        };

        const formatCost = (usd) => {
            if (usd === 0) return 'free';
            if (usd < 0.01) return `$${usd.toFixed(4)}`;
            return `$${usd.toFixed(2)}`;
        };

        const getModelCost = (model, totalSeconds) => {
            const pricePerMin = MODEL_PRICING[model];
            if (pricePerMin === undefined) return null;
            return (totalSeconds / 60) * pricePerMin;
        };

        const refreshStats = () => {
            if (!statsProxy) return;

            try {
                statsProxy.GetRecordingStatsRemote((result) => {
                    clearStatsRows();
                    statsGroup.remove(statsLoadingRow);

                    let stats;
                    try {
                        stats = JSON.parse(result[0]);
                    } catch (e) {
                        const errorRow = new Adw.ActionRow({
                            title: 'Error parsing stats',
                            subtitle: String(e),
                        });
                        statsGroup.add(errorRow);
                        statsRows.push(errorRow);
                        return;
                    }

                    const models = stats.models || {};
                    const modelKeys = Object.keys(models);

                    if (modelKeys.length === 0) {
                        const emptyRow = new Adw.ActionRow({
                            title: 'No recordings yet',
                            subtitle: 'Start recording to see statistics here',
                        });
                        statsGroup.add(emptyRow);
                        statsRows.push(emptyRow);
                    } else {
                        let totalTime = 0;
                        let totalCount = 0;
                        let totalCost = 0;
                        let allCostsKnown = true;

                        for (const model of modelKeys) {
                            const m = models[model];
                            totalTime += m.total_seconds;
                            totalCount += m.recording_count;

                            const cost = getModelCost(model, m.total_seconds);
                            if (cost !== null) {
                                totalCost += cost;
                            } else {
                                allCostsKnown = false;
                            }

                            const costStr = cost !== null ? ` \u00b7 ${formatCost(cost)}` : '';

                            const row = new Adw.ActionRow({
                                title: model,
                                subtitle: `${m.recording_count} recording${m.recording_count !== 1 ? 's' : ''}${costStr}`,
                            });

                            const label = new Gtk.Label({
                                label: formatDuration(m.total_seconds),
                                css_classes: ['dim-label'],
                                valign: Gtk.Align.CENTER,
                            });
                            row.add_suffix(label);
                            statsGroup.add(row);
                            statsRows.push(row);
                        }

                        // Total summary row
                        const costSuffix = allCostsKnown ? ` \u00b7 ${formatCost(totalCost)}` : '';
                        const totalRow = new Adw.ActionRow({
                            title: 'Total',
                            subtitle: `${totalCount} recording${totalCount !== 1 ? 's' : ''}${modelKeys.length > 1 ? ` across ${modelKeys.length} models` : ''}${costSuffix}`,
                        });
                        const totalLabel = new Gtk.Label({
                            label: formatDuration(totalTime),
                            css_classes: ['accent'],
                            valign: Gtk.Align.CENTER,
                        });
                        totalRow.add_suffix(totalLabel);
                        statsGroup.add(totalRow);
                        statsRows.push(totalRow);
                    }

                    // Reset button
                    const resetRow = new Adw.ActionRow();
                    const resetBtn = new Gtk.Button({
                        label: 'Reset Statistics',
                        halign: Gtk.Align.CENTER,
                        hexpand: true,
                        valign: Gtk.Align.CENTER,
                    });
                    resetBtn.add_css_class('destructive-action');
                    resetBtn.connect('clicked', () => {
                        statsProxy.ResetRecordingStatsRemote(() => {
                            refreshStats();
                        });
                    });
                    resetRow.set_child(resetBtn);
                    statsGroup.add(resetRow);
                    statsRows.push(resetRow);
                });
            } catch (e) {
                clearStatsRows();
                statsGroup.remove(statsLoadingRow);
                const errorRow = new Adw.ActionRow({
                    title: 'Cannot connect to daemon',
                    subtitle: 'Make sure voicify daemon is running',
                });
                statsGroup.add(errorRow);
                statsRows.push(errorRow);
            }
        };

        try {
            statsProxy = new StatsProxy(
                Gio.DBus.session,
                'com.dooshek.voicify',
                '/com/dooshek/voicify/Recorder'
            );
            refreshStats();
        } catch (e) {
            statsGroup.remove(statsLoadingRow);
            const errorRow = new Adw.ActionRow({
                title: 'Cannot connect to daemon',
                subtitle: 'Make sure voicify daemon is running',
            });
            statsGroup.add(errorRow);
            statsRows.push(errorRow);
        }

        // Cleanup on window close
        window.connect('close-request', () => {
            for (const id of settingsHandlerIds) {
                settings.disconnect(id);
            }
            settingsHandlerIds.length = 0;
            statsProxy = null;
            return false;
        });
    }
}

// Helper: add an editable shortcut row
function _addShortcutRow(group, title, settingKey, settings, window) {
    const row = new Adw.ActionRow({ title: title });

    const accel = settings.get_string(settingKey);
    const button = new Gtk.Button({
        label: _accelLabel(accel),
        valign: Gtk.Align.CENTER,
    });

    let capturing = false;
    let controller = null;

    button.connect('clicked', () => {
        if (capturing) return;
        capturing = true;
        button.set_label('Press shortcut...');
        button.add_css_class('suggested-action');

        controller = new Gtk.EventControllerKey();
        controller.connect('key-pressed', (ctrl, keyval, keycode, state) => {
            // Escape cancels
            if (keyval === Gdk.KEY_Escape) {
                button.set_label(_accelLabel(settings.get_string(settingKey)));
                button.remove_css_class('suggested-action');
                window.remove_controller(controller);
                controller = null;
                capturing = false;
                return true;
            }

            // Ignore standalone modifier keys
            if (keyval >= 0xffe1 && keyval <= 0xffee) return false;

            const mask = state & ACCEL_MODS;

            // Require at least Ctrl, Alt, or Super
            const requiredMods = Gdk.ModifierType.CONTROL_MASK |
                                 Gdk.ModifierType.ALT_MASK |
                                 Gdk.ModifierType.SUPER_MASK;
            if ((mask & requiredMods) === 0) {
                return true;
            }

            const accelName = Gtk.accelerator_name(keyval, mask);
            if (accelName) {
                settings.set_string(settingKey, accelName);
                button.set_label(_accelLabel(accelName));
            }

            button.remove_css_class('suggested-action');
            window.remove_controller(controller);
            controller = null;
            capturing = false;
            return true;
        });

        window.add_controller(controller);
    });

    // Update button when setting changes externally
    const handlerId = settings.connect(`changed::${settingKey}`, () => {
        if (!capturing) {
            button.set_label(_accelLabel(settings.get_string(settingKey)));
        }
    });

    row.connect('destroy', () => {
        settings.disconnect(handlerId);
        if (controller) {
            window.remove_controller(controller);
            controller = null;
        }
    });

    row.add_suffix(button);
    group.add(row);
}

// --- Distro detection for install hints ---

function _getInstallCommand() {
    try {
        const [ok, contents] = GLib.file_get_contents('/etc/os-release');
        if (!ok) return 'sudo dnf install playerctl';
        const text = new TextDecoder().decode(contents);
        const idMatch = text.match(/^ID=(.+)$/m);
        const id = idMatch ? idMatch[1].replace(/"/g, '').toLowerCase() : '';

        if (['fedora', 'rhel', 'centos', 'nobara'].includes(id))
            return 'sudo dnf install playerctl';
        if (['ubuntu', 'debian', 'pop', 'linuxmint', 'elementary', 'zorin'].includes(id))
            return 'sudo apt install playerctl';
        if (['arch', 'manjaro', 'endeavouros', 'garuda', 'cachyos'].includes(id))
            return 'sudo pacman -S playerctl';
        if (['opensuse-tumbleweed', 'opensuse-leap', 'suse'].includes(id))
            return 'sudo zypper install playerctl';
        if (id === 'gentoo')
            return 'sudo emerge media-sound/playerctl';
        if (id === 'void')
            return 'sudo xbps-install playerctl';
        if (id === 'nixos')
            return 'nix-env -iA nixpkgs.playerctl';
    } catch (e) {
        // ignore
    }
    return 'sudo dnf install playerctl';
}

// --- Advanced settings helpers ---

function _getOverride(settings, designId, path, defaultVal) {
    let all;
    try {
        all = JSON.parse(settings.get_string('design-overrides') || '{}');
    } catch (e) {
        all = {};
    }
    const o = all[designId];
    if (!o) return defaultVal;

    const parts = path.split('.');
    let cur = o;
    for (const p of parts) {
        if (cur === undefined || cur === null) return defaultVal;
        cur = cur[p];
    }
    return cur !== undefined ? cur : defaultVal;
}

function _setOverride(settings, designId, path, value) {
    let all;
    try {
        all = JSON.parse(settings.get_string('design-overrides') || '{}');
    } catch (e) {
        all = {};
    }
    if (!all[designId]) all[designId] = {};

    const parts = path.split('.');
    let cur = all[designId];
    for (let i = 0; i < parts.length - 1; i++) {
        if (!cur[parts[i]] || typeof cur[parts[i]] !== 'object') cur[parts[i]] = {};
        cur = cur[parts[i]];
    }
    cur[parts[parts.length - 1]] = value;

    settings.set_string('design-overrides', JSON.stringify(all));
}

function _deleteOverrides(settings, designId) {
    let all;
    try {
        all = JSON.parse(settings.get_string('design-overrides') || '{}');
    } catch (e) {
        all = {};
    }
    delete all[designId];
    settings.set_string('design-overrides', JSON.stringify(all));
}

function _buildAdvancedGroups(settings, designs, designId) {
    const design = designs.get(designId);
    if (!design) return [];

    const groups = [];

    // --- Container group ---
    const containerGroup = new Adw.PreferencesGroup({ title: 'Container' });
    groups.push(containerGroup);

    const dc = design.container;

    // Border Radius
    const contRadiusRow = new Adw.SpinRow({
        title: 'Border Radius',
        adjustment: new Gtk.Adjustment({
            lower: 0, upper: 60, step_increment: 1, page_increment: 5,
            value: _getOverride(settings, designId, 'container.borderRadius', dc.borderRadius),
        }),
    });
    contRadiusRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'container.borderRadius', contRadiusRow.get_value());
    });
    containerGroup.add(contRadiusRow);

    // Background Opacity
    const contOpacityRow = new Adw.SpinRow({
        title: 'Background Opacity',
        digits: 2,
        adjustment: new Gtk.Adjustment({
            lower: 0.0, upper: 1.0, step_increment: 0.05, page_increment: 0.1,
            value: _getOverride(settings, designId, 'container.bgOpacity', dc.bgOpacity),
        }),
    });
    contOpacityRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'container.bgOpacity', contOpacityRow.get_value());
    });
    containerGroup.add(contOpacityRow);

    // Background Color
    const bgColor = _getOverride(settings, designId, 'container.bgColor', dc.bgColor);
    const bgColorRow = new Adw.ActionRow({ title: 'Background Color' });
    const bgRgba = new Gdk.RGBA();
    bgRgba.red = (bgColor[0] || 0) / 255;
    bgRgba.green = (bgColor[1] || 0) / 255;
    bgRgba.blue = (bgColor[2] || 0) / 255;
    bgRgba.alpha = 1.0;
    const bgColorDialog = new Gtk.ColorDialog();
    const bgColorBtn = new Gtk.ColorDialogButton({
        dialog: bgColorDialog,
        rgba: bgRgba,
        valign: Gtk.Align.CENTER,
    });
    bgColorBtn.connect('notify::rgba', () => {
        const c = bgColorBtn.get_rgba();
        _setOverride(settings, designId, 'container.bgColor', [
            Math.round(c.red * 255),
            Math.round(c.green * 255),
            Math.round(c.blue * 255),
        ]);
    });
    bgColorRow.add_suffix(bgColorBtn);
    containerGroup.add(bgColorRow);

    // --- Bars group ---
    const barsGroup = new Adw.PreferencesGroup({ title: 'Bars' });
    groups.push(barsGroup);

    const db = design.bars;

    // Border Radius
    const barRadiusRow = new Adw.SpinRow({
        title: 'Border Radius',
        adjustment: new Gtk.Adjustment({
            lower: 0, upper: 20, step_increment: 1, page_increment: 2,
            value: _getOverride(settings, designId, 'bars.borderRadius', db.borderRadius),
        }),
    });
    barRadiusRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'bars.borderRadius', barRadiusRow.get_value());
    });
    barsGroup.add(barRadiusRow);

    // Scale Min
    const scaleMinRow = new Adw.SpinRow({
        title: 'Scale Min',
        digits: 2,
        adjustment: new Gtk.Adjustment({
            lower: 0.0, upper: 1.0, step_increment: 0.02, page_increment: 0.1,
            value: _getOverride(settings, designId, 'bars.scaleMin', db.scaleMin),
        }),
    });
    scaleMinRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'bars.scaleMin', scaleMinRow.get_value());
    });
    barsGroup.add(scaleMinRow);

    // Scale Max
    const scaleMaxRow = new Adw.SpinRow({
        title: 'Scale Max',
        digits: 2,
        adjustment: new Gtk.Adjustment({
            lower: 0.1, upper: 2.0, step_increment: 0.05, page_increment: 0.1,
            value: _getOverride(settings, designId, 'bars.scaleMax', db.scaleMax),
        }),
    });
    scaleMaxRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'bars.scaleMax', scaleMaxRow.get_value());
    });
    barsGroup.add(scaleMaxRow);

    // Opacity Mode
    const OPACITY_MODES = ['uniform', 'gradient'];
    const OPACITY_LABELS = ['Uniform', 'Gradient'];
    const opacityModeModel = new Gtk.StringList();
    for (const l of OPACITY_LABELS) opacityModeModel.append(l);

    const curOpacityMode = _getOverride(settings, designId, 'bars.opacityMode', db.opacityMode);

    const opacityModeRow = new Adw.ComboRow({
        title: 'Opacity Mode',
        model: opacityModeModel,
    });
    opacityModeRow.set_selected(OPACITY_MODES.indexOf(curOpacityMode));

    // Opacity Uniform (visible when mode=uniform)
    const opUniformRow = new Adw.SpinRow({
        title: 'Opacity',
        adjustment: new Gtk.Adjustment({
            lower: 0, upper: 255, step_increment: 5, page_increment: 25,
            value: _getOverride(settings, designId, 'bars.opacityUniform', db.opacityUniform),
        }),
    });
    opUniformRow.visible = (curOpacityMode === 'uniform');
    opUniformRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'bars.opacityUniform', opUniformRow.get_value());
    });

    // Opacity Min/Max (visible when mode=gradient)
    const opMinRow = new Adw.SpinRow({
        title: 'Opacity Min',
        adjustment: new Gtk.Adjustment({
            lower: 0, upper: 255, step_increment: 5, page_increment: 25,
            value: _getOverride(settings, designId, 'bars.opacityMin', db.opacityMin),
        }),
    });
    opMinRow.visible = (curOpacityMode === 'gradient');
    opMinRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'bars.opacityMin', opMinRow.get_value());
    });

    const opMaxRow = new Adw.SpinRow({
        title: 'Opacity Max',
        adjustment: new Gtk.Adjustment({
            lower: 0, upper: 255, step_increment: 5, page_increment: 25,
            value: _getOverride(settings, designId, 'bars.opacityMax', db.opacityMax),
        }),
    });
    opMaxRow.visible = (curOpacityMode === 'gradient');
    opMaxRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'bars.opacityMax', opMaxRow.get_value());
    });

    opacityModeRow.connect('notify::selected', () => {
        const idx = opacityModeRow.get_selected();
        if (idx >= 0 && idx < OPACITY_MODES.length) {
            const mode = OPACITY_MODES[idx];
            _setOverride(settings, designId, 'bars.opacityMode', mode);
            opUniformRow.visible = (mode === 'uniform');
            opMinRow.visible = (mode === 'gradient');
            opMaxRow.visible = (mode === 'gradient');
        }
    });

    barsGroup.add(opacityModeRow);
    barsGroup.add(opUniformRow);
    barsGroup.add(opMinRow);
    barsGroup.add(opMaxRow);

    // Glow Radius
    const glowRadiusRow = new Adw.SpinRow({
        title: 'Glow Radius',
        adjustment: new Gtk.Adjustment({
            lower: 0, upper: 20, step_increment: 1, page_increment: 2,
            value: _getOverride(settings, designId, 'bars.glowRadius', db.glowRadius),
        }),
    });
    glowRadiusRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'bars.glowRadius', glowRadiusRow.get_value());
    });
    barsGroup.add(glowRadiusRow);

    // Glow Alpha
    const glowAlphaRow = new Adw.SpinRow({
        title: 'Glow Alpha',
        digits: 2,
        adjustment: new Gtk.Adjustment({
            lower: 0.0, upper: 1.0, step_increment: 0.05, page_increment: 0.1,
            value: _getOverride(settings, designId, 'bars.glowAlpha', db.glowAlpha),
        }),
    });
    glowAlphaRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'bars.glowAlpha', glowAlphaRow.get_value());
    });
    barsGroup.add(glowAlphaRow);

    // Highlight
    const highlightRow = new Adw.SwitchRow({
        title: 'Highlight',
        subtitle: 'Top highlight on each bar',
        active: _getOverride(settings, designId, 'bars.highlight', db.highlight),
    });
    highlightRow.connect('notify::active', () => {
        _setOverride(settings, designId, 'bars.highlight', highlightRow.get_active());
    });
    barsGroup.add(highlightRow);

    // Color Mute
    const colorMuteRow = new Adw.SpinRow({
        title: 'Color Mute',
        digits: 2,
        adjustment: new Gtk.Adjustment({
            lower: 0.0, upper: 1.0, step_increment: 0.05, page_increment: 0.1,
            value: _getOverride(settings, designId, 'bars.colorMute', db.colorMute),
        }),
    });
    colorMuteRow.connect('notify::value', () => {
        _setOverride(settings, designId, 'bars.colorMute', colorMuteRow.get_value());
    });
    barsGroup.add(colorMuteRow);

    // --- Layers group ---
    const layers = design.layers || [];
    if (layers.length > 0) {
        const layersGroup = new Adw.PreferencesGroup({ title: 'Layers' });
        groups.push(layersGroup);

        for (const layer of layers) {
            const layerType = layer.type;
            const capitalType = layerType.charAt(0).toUpperCase() + layerType.slice(1);

            const expander = new Adw.ExpanderRow({
                title: capitalType,
                subtitle: `${layerType} layer`,
            });
            layersGroup.add(expander);

            // Enable/disable
            const layerEnabled = _getOverride(settings, designId, `layers.${layerType}.enabled`, true);
            const enableRow = new Adw.SwitchRow({
                title: 'Enabled',
                active: layerEnabled !== false,
            });
            enableRow.connect('notify::active', () => {
                _setOverride(settings, designId, `layers.${layerType}.enabled`, enableRow.get_active());
            });
            expander.add_row(enableRow);

            // Layer-specific params
            if (layerType === 'shadow') {
                _addLayerSpinRow(expander, 'Blur', 0, 30, 1, layer.blur || 8, settings, designId, `layers.${layerType}.blur`);
                _addLayerSpinRowFloat(expander, 'Alpha', 0, 1, 0.05, layer.alpha || 0.3, settings, designId, `layers.${layerType}.alpha`);
                _addLayerSpinRow(expander, 'Offset X', -20, 20, 1, layer.x || 0, settings, designId, `layers.${layerType}.x`);
                _addLayerSpinRow(expander, 'Offset Y', -20, 20, 1, layer.y || 0, settings, designId, `layers.${layerType}.y`);
            } else if (layerType === 'border') {
                _addLayerSpinRow(expander, 'Width', 0, 10, 1, layer.width || 2, settings, designId, `layers.${layerType}.width`);
                _addLayerSpinRowFloat(expander, 'Alpha', 0, 1, 0.05, layer.alpha || 0.3, settings, designId, `layers.${layerType}.alpha`);
            } else if (layerType === 'pixelGrid') {
                _addLayerSpinRowFloat(expander, 'Alpha', 0, 1, 0.02, layer.alpha || 0.12, settings, designId, `layers.${layerType}.alpha`);
            } else if (layerType === 'scanlines') {
                _addLayerSpinRow(expander, 'Line Height', 1, 5, 1, layer.lineHeight || 1, settings, designId, `layers.${layerType}.lineHeight`);
                _addLayerSpinRow(expander, 'Line Spacing', 1, 10, 1, layer.lineSpacing || 3, settings, designId, `layers.${layerType}.lineSpacing`);
                _addLayerSpinRowFloat(expander, 'Alpha', 0, 1, 0.05, layer.alpha || 0.15, settings, designId, `layers.${layerType}.alpha`);
            } else if (layerType === 'frame') {
                _addLayerSpinRow(expander, 'Border Width', 0, 10, 1, layer.borderWidth || 2, settings, designId, `layers.${layerType}.borderWidth`);
                _addLayerSpinRow(expander, 'Border Radius', 0, 40, 1, layer.borderRadius || 20, settings, designId, `layers.${layerType}.borderRadius`);
                _addLayerSpinRowFloat(expander, 'Alpha', 0, 1, 0.05, layer.alpha || 0.7, settings, designId, `layers.${layerType}.alpha`);
                _addLayerSpinRow(expander, 'Inset', 0, 20, 1, layer.inset || 0, settings, designId, `layers.${layerType}.inset`);
            } else if (layerType === 'innerHighlight' || layerType === 'specularHighlight' || layerType === 'innerShadow') {
                _addLayerSpinRowFloat(expander, 'Alpha', 0, 1, 0.05, layer.alpha || 0.1, settings, designId, `layers.${layerType}.alpha`);
            }
        }
    }

    // --- Reset button ---
    const resetGroup = new Adw.PreferencesGroup();
    groups.push(resetGroup);

    const resetRow = new Adw.ActionRow();
    const resetBtn = new Gtk.Button({
        label: 'Reset to Defaults',
        halign: Gtk.Align.CENTER,
        hexpand: true,
        valign: Gtk.Align.CENTER,
    });
    resetBtn.add_css_class('destructive-action');
    resetBtn.connect('clicked', () => {
        _deleteOverrides(settings, designId);
    });
    resetRow.set_child(resetBtn);
    resetGroup.add(resetRow);

    return groups;
}

function _addLayerSpinRow(parent, title, lower, upper, step, defaultVal, settings, designId, path) {
    const curVal = _getOverride(settings, designId, path, defaultVal);
    const row = new Adw.SpinRow({
        title: title,
        adjustment: new Gtk.Adjustment({
            lower, upper, step_increment: step, page_increment: step * 5,
            value: curVal,
        }),
    });
    row.connect('notify::value', () => {
        _setOverride(settings, designId, path, row.get_value());
    });
    parent.add_row(row);
}

function _addLayerSpinRowFloat(parent, title, lower, upper, step, defaultVal, settings, designId, path) {
    const curVal = _getOverride(settings, designId, path, defaultVal);
    const row = new Adw.SpinRow({
        title: title,
        digits: 2,
        adjustment: new Gtk.Adjustment({
            lower, upper, step_increment: step, page_increment: step * 5,
            value: curVal,
        }),
    });
    row.connect('notify::value', () => {
        _setOverride(settings, designId, path, row.get_value());
    });
    parent.add_row(row);
}

// Helper: accelerator string to human-readable label
function _accelLabel(accel) {
    if (!accel) return '';
    const [parsed, keyval, mods] = Gtk.accelerator_parse(accel);
    if (parsed) {
        return Gtk.accelerator_get_label(keyval, mods);
    }
    // Fallback: manual format
    return accel
        .replace(/<Primary>/gi, 'Ctrl+')
        .replace(/<Control>/gi, 'Ctrl+')
        .replace(/<Ctrl>/gi, 'Ctrl+')
        .replace(/<Shift>/gi, 'Shift+')
        .replace(/<Alt>/gi, 'Alt+')
        .replace(/<Super>/gi, 'Super+');
}
