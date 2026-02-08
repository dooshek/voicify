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
            title: 'Style',
            description: 'Use keyboard shortcuts to start recording and see the visualization live',
        });
        page.add(styleGroup);

        // Design style combo (first)
        const designModel = new Gtk.StringList();
        for (const id of DESIGN_IDS) {
            designModel.append(designs.get(id).name);
        }

        const designRow = new Adw.ComboRow({
            title: 'Design',
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

        // --- Position group ---
        const positionGroup = new Adw.PreferencesGroup({
            title: 'Position',
            description: 'Drag the waveform widget during recording to reposition it',
        });
        page.add(positionGroup);

        // --- Keyboard shortcuts group (editable) ---
        const shortcutsGroup = new Adw.PreferencesGroup({
            title: 'Keyboard Shortcuts',
            description: 'Click a shortcut to change it',
        });
        page.add(shortcutsGroup);

        _addShortcutRow(shortcutsGroup, 'Realtime', 'shortcut-realtime', settings, window);
        _addShortcutRow(shortcutsGroup, 'Post + auto-paste', 'shortcut-post-autopaste', settings, window);
        _addShortcutRow(shortcutsGroup, 'Post + router', 'shortcut-post-router', settings, window);
        _addShortcutRow(shortcutsGroup, 'Cancel', 'shortcut-cancel', settings, window);

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

        // Cleanup on window close
        window.connect('close-request', () => {
            for (const id of settingsHandlerIds) {
                settings.disconnect(id);
            }
            settingsHandlerIds.length = 0;
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
