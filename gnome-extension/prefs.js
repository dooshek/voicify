'use strict';

import Adw from 'gi://Adw';
import Gtk from 'gi://Gtk';
import Gdk from 'gi://Gdk';
import GLib from 'gi://GLib';
import Gio from 'gi://Gio';

import { ExtensionPreferences } from 'resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js';

// Duplicated from extension.js (prefs.js runs in a different process)
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

const THEME_IDS = Object.keys(THEMES);

const SIZES = {
    'thin':   { barWidth: 2, barSpacing: 2, numBars: 32, containerHeight: 40 },
    'medium': { barWidth: 4, barSpacing: 3, numBars: 26, containerHeight: 56 },
    'large':  { barWidth: 6, barSpacing: 4, numBars: 20, containerHeight: 72 },
};

const SIZE_IDS = ['thin', 'medium', 'large'];

const POSITIONS = [
    'top-left', 'top-center', 'top-right',
    'middle-left', 'middle-center', 'middle-right',
    'bottom-left', 'bottom-center', 'bottom-right',
];

// Modifier mask for accelerator capture
const ACCEL_MODS = Gdk.ModifierType.CONTROL_MASK |
                   Gdk.ModifierType.SHIFT_MASK |
                   Gdk.ModifierType.ALT_MASK |
                   Gdk.ModifierType.SUPER_MASK;

export default class VoicifyPreferences extends ExtensionPreferences {
    fillPreferencesWindow(window) {
        const settings = this.getSettings();
        let animTimerId = null;
        const settingsHandlerIds = [];

        const page = new Adw.PreferencesPage({
            title: 'Appearance',
            icon_name: 'preferences-desktop-appearance-symbolic',
        });
        window.add(page);

        // --- Theme group ---
        const themeGroup = new Adw.PreferencesGroup({
            title: 'Theme',
            description: 'Choose a color theme for the waveform visualization',
        });
        page.add(themeGroup);

        const themeModel = new Gtk.StringList();
        for (const id of THEME_IDS) {
            themeModel.append(THEMES[id].name);
        }

        const themeRow = new Adw.ComboRow({
            title: 'Color Theme',
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
        themeGroup.add(themeRow);

        // --- Size (created early so drawPreview can reference sizeRow) ---
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

        // --- Theme preview (Cairo animated) ---
        const previewRow = new Adw.ActionRow({
            title: 'Preview',
        });

        const previewArea = new Gtk.DrawingArea({
            content_width: 280,
            content_height: 80,
            valign: Gtk.Align.CENTER,
        });

        let animPhase = 0;

        const drawPreview = (area, cr, width, height) => {
            const themeIdx = themeRow.get_selected();
            const themeId = THEME_IDS[themeIdx] || 'mint-dream';
            const theme = THEMES[themeId];

            const sizeIdx = sizeRow.get_selected();
            const sizeId = SIZE_IDS[sizeIdx] || 'medium';
            const size = SIZES[sizeId];

            const { barWidth, barSpacing, numBars, containerHeight } = size;
            const totalWidth = numBars * (barWidth + barSpacing);
            const barHeight = (containerHeight / 2) + 1;
            const offsetX = (width - totalWidth) / 2;
            const centerY = height / 2;

            const bgRadius = 12;
            cr.setSourceRGBA(0.12, 0.12, 0.14, 1);
            _roundedRect(cr, 0, 0, width, height, bgRadius);
            cr.fill();

            const barCenter = (numBars - 1) / 2;

            for (let i = 0; i < numBars; i++) {
                const dist = Math.abs(i - barCenter) / barCenter;
                const r = (theme.center.r + (theme.edge.r - theme.center.r) * dist) / 255;
                const g = (theme.center.g + (theme.edge.g - theme.center.g) * dist) / 255;
                const b = (theme.center.b + (theme.edge.b - theme.center.b) * dist) / 255;
                const alpha = 0.3 + 0.7 * (1 - dist);

                const envelope = Math.exp(-dist * dist * 3);
                const wave = Math.sin(animPhase + i * 0.3) * 0.5 + 0.5;
                const scale = (0.2 + wave * 0.8) * envelope;
                const h = Math.max(2, barHeight * scale);

                const x = offsetX + i * (barWidth + barSpacing);

                cr.setSourceRGBA(r, g, b, alpha);

                const barRadius = Math.min(barWidth / 2, 2);
                _roundedRect(cr, x, centerY - h, barWidth, h, barRadius);
                cr.fill();

                _roundedRect(cr, x, centerY, barWidth, h, barRadius);
                cr.fill();
            }
        };

        previewArea.set_draw_func(drawPreview);
        previewRow.add_suffix(previewArea);
        themeGroup.add(previewRow);

        animTimerId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 60, () => {
            animPhase += 0.08;
            previewArea.queue_draw();
            return GLib.SOURCE_CONTINUE;
        });

        // Background opacity slider
        const bgOpacityRow = new Adw.ActionRow({
            title: 'Background',
            subtitle: 'Opacity of the dark backdrop behind bars',
        });

        const bgAdj = new Gtk.Adjustment({
            lower: 0,
            upper: 100,
            step_increment: 1,
            page_increment: 10,
            value: settings.get_int('wave-bg-opacity'),
        });

        const bgScale = new Gtk.Scale({
            adjustment: bgAdj,
            orientation: Gtk.Orientation.HORIZONTAL,
            draw_value: false,
            hexpand: true,
            valign: Gtk.Align.CENTER,
            width_request: 200,
        });

        bgScale.connect('value-changed', () => {
            settings.set_int('wave-bg-opacity', Math.round(bgScale.get_value()));
        });

        bgOpacityRow.add_suffix(bgScale);
        themeGroup.add(bgOpacityRow);

        // --- Size group (add to UI) ---
        const sizeGroup = new Adw.PreferencesGroup({
            title: 'Size',
            description: 'Adjust the waveform bar dimensions',
        });
        page.add(sizeGroup);

        sizeRow.connect('notify::selected', () => {
            const idx = sizeRow.get_selected();
            if (idx >= 0 && idx < SIZE_IDS.length) {
                settings.set_string('wave-size', SIZE_IDS[idx]);
                previewArea.queue_draw();
            }
        });
        sizeGroup.add(sizeRow);

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
            description: 'Choose where the waveform appears on screen',
        });
        page.add(positionGroup);

        const positionRow = new Adw.ActionRow({
            title: 'Screen Position',
        });

        const positionArea = new Gtk.DrawingArea({
            content_width: 240,
            content_height: 180,
            valign: Gtk.Align.CENTER,
        });

        let currentPosition = settings.get_string('wave-position');

        const drawPosition = (area, cr, width, height) => {
            const padding = 8;
            const screenW = width - padding * 2;
            const screenH = height - padding * 2;
            const screenX = padding;
            const screenY = padding;
            const screenRadius = 10;

            cr.setSourceRGBA(0.3, 0.3, 0.35, 1);
            _roundedRect(cr, screenX, screenY, screenW, screenH, screenRadius);
            cr.fill();

            cr.setSourceRGBA(0.15, 0.15, 0.18, 1);
            _roundedRect(cr, screenX + 2, screenY + 2, screenW - 4, screenH - 4, screenRadius - 1);
            cr.fill();

            cr.setSourceRGBA(0.2, 0.2, 0.24, 1);
            cr.rectangle(screenX + 2, screenY + 2, screenW - 4, 12);
            cr.fill();

            const zoneW = (screenW - 4) / 3;
            const panelH = 14;
            const zoneH = (screenH - 4 - panelH) / 3;

            for (let row = 0; row < 3; row++) {
                for (let col = 0; col < 3; col++) {
                    const posIdx = row * 3 + col;
                    const posId = POSITIONS[posIdx];
                    const isActive = (posId === currentPosition);

                    const zx = screenX + 2 + col * zoneW;
                    const zy = screenY + 2 + panelH + row * zoneH;

                    if (isActive) {
                        cr.setSourceRGBA(0.24, 0.47, 0.87, 0.35);
                        cr.rectangle(zx, zy, zoneW, zoneH);
                        cr.fill();
                    }

                    const miniBarW = 3;
                    const miniBarH = [6, 10, 14, 10, 6];
                    const miniSpacing = 2;
                    const miniTotalW = 5 * miniBarW + 4 * miniSpacing;
                    const miniCx = zx + zoneW / 2 - miniTotalW / 2;
                    const miniCy = zy + zoneH / 2;

                    for (let b = 0; b < 5; b++) {
                        const bx = miniCx + b * (miniBarW + miniSpacing);
                        const bh = miniBarH[b];

                        if (isActive) {
                            cr.setSourceRGBA(0.35, 0.6, 1, 0.9);
                        } else {
                            cr.setSourceRGBA(0.5, 0.5, 0.55, 0.5);
                        }
                        _roundedRect(cr, bx, miniCy - bh / 2, miniBarW, bh, 1);
                        cr.fill();
                    }
                }
            }

            cr.setSourceRGBA(0.4, 0.4, 0.45, 0.3);
            cr.setLineWidth(0.5);
            for (let col = 1; col < 3; col++) {
                const lx = screenX + 2 + col * zoneW;
                cr.moveTo(lx, screenY + 2 + panelH);
                cr.lineTo(lx, screenY + screenH - 2);
                cr.stroke();
            }
            for (let row = 1; row < 3; row++) {
                const ly = screenY + 2 + panelH + row * zoneH;
                cr.moveTo(screenX + 2, ly);
                cr.lineTo(screenX + screenW - 2, ly);
                cr.stroke();
            }
        };

        positionArea.set_draw_func(drawPosition);

        const clickGesture = new Gtk.GestureClick();
        clickGesture.connect('pressed', (gesture, nPress, x, y) => {
            const areaWidth = positionArea.get_width();
            const areaHeight = positionArea.get_height();
            const padding = 8;
            const screenW = areaWidth - padding * 2;
            const screenH = areaHeight - padding * 2;
            const screenX = padding;
            const screenY = padding;
            const panelH = 14;
            const zoneW = (screenW - 4) / 3;
            const zoneH = (screenH - 4 - panelH) / 3;

            const relX = x - screenX - 2;
            const relY = y - screenY - 2 - panelH;

            if (relX < 0 || relY < 0 || relX >= zoneW * 3 || relY >= zoneH * 3) return;

            const col = Math.floor(relX / zoneW);
            const row = Math.floor(relY / zoneH);
            const posIdx = row * 3 + col;

            if (posIdx >= 0 && posIdx < POSITIONS.length) {
                currentPosition = POSITIONS[posIdx];
                settings.set_string('wave-position', currentPosition);
                positionArea.queue_draw();
            }
        });
        positionArea.add_controller(clickGesture);

        positionRow.add_suffix(positionArea);
        positionGroup.add(positionRow);

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
                previewArea.queue_draw();
            })
        );

        settingsHandlerIds.push(
            settings.connect('changed::wave-position', () => {
                currentPosition = settings.get_string('wave-position');
                positionArea.queue_draw();
            })
        );

        // Cleanup on window close
        window.connect('close-request', () => {
            if (animTimerId !== null) {
                GLib.Source.remove(animTimerId);
                animTimerId = null;
            }
            for (const id of settingsHandlerIds) {
                settings.disconnect(id);
            }
            settingsHandlerIds.length = 0;
            return false;
        });
    }
}

// Helper: draw a rounded rectangle
function _roundedRect(cr, x, y, w, h, r) {
    r = Math.min(r, w / 2, h / 2);
    cr.newPath();
    cr.arc(x + r, y + r, r, Math.PI, 1.5 * Math.PI);
    cr.arc(x + w - r, y + r, r, 1.5 * Math.PI, 2 * Math.PI);
    cr.arc(x + w - r, y + h - r, r, 0, 0.5 * Math.PI);
    cr.arc(x + r, y + h - r, r, 0.5 * Math.PI, Math.PI);
    cr.closePath();
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
                // No real modifier pressed - ignore
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
