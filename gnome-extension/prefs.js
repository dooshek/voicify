'use strict';

import Adw from 'gi://Adw';
import Gtk from 'gi://Gtk';
import Gdk from 'gi://Gdk';
import GLib from 'gi://GLib';
import Gio from 'gi://Gio';

import { ExtensionPreferences } from 'resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js';
import { loadDesigns, getDesignIds } from './designLoader.js';
import * as LayerPainter from './layerPainter.js';

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

        const designs = loadDesigns(this.path);
        const DESIGN_IDS = getDesignIds(designs);

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

        // Design style combo
        const designModel = new Gtk.StringList();
        for (const id of DESIGN_IDS) {
            designModel.append(designs.get(id).name);
        }

        const designRow = new Adw.ComboRow({
            title: 'Design Style',
            subtitle: 'Visual rendering style for the waveform',
            model: designModel,
        });

        const currentDesignId = settings.get_string('wave-design');
        const currentDesignIdx = DESIGN_IDS.indexOf(currentDesignId);
        if (currentDesignIdx >= 0) designRow.set_selected(currentDesignIdx);

        designRow.connect('notify::selected', () => {
            const idx = designRow.get_selected();
            if (idx >= 0 && idx < DESIGN_IDS.length) {
                const id = DESIGN_IDS[idx];
                settings.set_string('wave-design', id);
                const design = designs.get(id);
                if (design && design.defaultTheme && THEMES[design.defaultTheme]) {
                    settings.set_string('wave-theme', design.defaultTheme);
                }
                previewArea.queue_draw();
            }
        });
        themeGroup.add(designRow);

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

        // --- Wave type ---
        const WAVE_TYPES = ['default', 'bottom', 'center', 'top', 'wave'];
        const WAVE_TYPE_LABELS = ['Default (design)', 'From Bottom', 'From Center', 'From Top', 'Wave (propagating)'];

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
                previewArea.queue_draw();
            }
        });

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

            const designIdx = designRow.get_selected();
            const designId = DESIGN_IDS[designIdx] || 'modern';
            const design = designs.get(designId) || designs.values().next().value;
            const dc = design.container;
            const db = design.bars;

            const sizeIdx = sizeRow.get_selected();
            const sizeId = SIZE_IDS[sizeIdx] || 'medium';
            const size = SIZES[sizeId];

            const { barWidth, barSpacing, numBars, containerHeight } = size;
            const widthAdj = db.widthAdjust || 0;
            const effectiveBarWidth = barWidth + widthAdj;
            const totalBarWidth = numBars * (effectiveBarWidth + barSpacing) - barSpacing;
            const barHeight = (containerHeight / 2) + 1;

            // Container dimensions matching actual widget
            const hPad = 14;
            const vPad = 10;
            const containerW = Math.min(width - 8, totalBarWidth + hPad * 2);
            const containerH = Math.min(height - 8, containerHeight + vPad * 2);
            const containerX = (width - containerW) / 2;
            const containerY = (height - containerH) / 2;
            const offsetX = containerX + (containerW - totalBarWidth) / 2;
            const centerY = containerY + containerH / 2;

            // Blur hint (frosted background simulation for Glass design)
            if (dc.blur) {
                const bgRadius = Math.min(containerH / 2, dc.borderRadius * (containerH / 76));
                cr.setSourceRGBA(0.7, 0.7, 0.75, 0.35);
                _roundedRect(cr, containerX, containerY, containerW, containerH, bgRadius);
                cr.fill();
            }

            // Draw all canvas layers (shadow, background, innerHighlight, specularHighlight, innerShadow, border)
            LayerPainter.drawAllCanvasLayersAt(cr, design, theme, containerX, containerY, containerW, containerH);

            // Bar colors
            let barCenter_c, barEdge_c;
            if (db.colorOverride) {
                const co = db.colorOverride;
                barCenter_c = { r: co.center[0], g: co.center[1], b: co.center[2] };
                barEdge_c = { r: co.edge[0], g: co.edge[1], b: co.edge[2] };
            } else {
                barCenter_c = { ...theme.center };
                barEdge_c = { ...theme.edge };
            }

            const mute = db.colorMute || 0;
            if (mute > 0) {
                const gray = { r: 160, g: 160, b: 160 };
                barCenter_c = _blendColor(barCenter_c, gray, mute);
                barEdge_c = _blendColor(barEdge_c, gray, mute);
            }

            const barCenter = (numBars - 1) / 2;
            const barRadius = Math.min(effectiveBarWidth / 2, db.borderRadius);
            const pivotY = db.pivotY;

            // Trail bars (drawn behind main bars when modifier > 0)
            const modifier = settings.get_int('wave-modifier') / 100;
            if (modifier > 0) {
                const trailAlpha = modifier * 0.45;
                const trailPhase = animPhase - 0.6;
                for (let i = 0; i < numBars; i++) {
                    const dist = Math.abs(i - barCenter) / barCenter;
                    const r = (barCenter_c.r + (barEdge_c.r - barCenter_c.r) * dist) / 255 * 0.7;
                    const g = (barCenter_c.g + (barEdge_c.g - barCenter_c.g) * dist) / 255 * 0.7;
                    const b = (barCenter_c.b + (barEdge_c.b - barCenter_c.b) * dist) / 255 * 0.7;

                    const envelope = Math.exp(-dist * dist * 3);
                    const wave = Math.sin(trailPhase + i * 0.3) * 0.5 + 0.5;
                    const scale = (0.3 + wave * 0.7) * envelope;
                    const h = Math.max(2, barHeight * scale);
                    const x = offsetX + i * (effectiveBarWidth + barSpacing);

                    let barY, barH;
                    if (pivotY === 1.0) {
                        const barBottom = containerY + containerH - 4;
                        barH = h * 2;
                        barY = barBottom - barH;
                    } else {
                        barH = h * 2;
                        barY = centerY - h;
                    }

                    cr.setSourceRGBA(r, g, b, trailAlpha);
                    _roundedRect(cr, x, barY, effectiveBarWidth, barH, barRadius);
                    cr.fill();
                }
            }

            for (let i = 0; i < numBars; i++) {
                const dist = Math.abs(i - barCenter) / barCenter;
                const r = (barCenter_c.r + (barEdge_c.r - barCenter_c.r) * dist) / 255;
                const g = (barCenter_c.g + (barEdge_c.g - barCenter_c.g) * dist) / 255;
                const b = (barCenter_c.b + (barEdge_c.b - barCenter_c.b) * dist) / 255;

                let alpha;
                if (db.opacityMode === 'uniform') {
                    alpha = (db.opacityUniform || 255) / 255;
                } else {
                    const oMin = (db.opacityMin || 80) / 255;
                    const oMax = (db.opacityMax || 255) / 255;
                    alpha = oMin + (oMax - oMin) * (1 - dist);
                }

                const envelope = Math.exp(-dist * dist * 3);
                const wave = Math.sin(animPhase + i * 0.3) * 0.5 + 0.5;
                const scale = (0.2 + wave * 0.8) * envelope;
                const h = Math.max(2, barHeight * scale);

                const x = offsetX + i * (effectiveBarWidth + barSpacing);

                // Bar position based on pivot mode
                let barY, barH;
                if (pivotY === 1.0) {
                    // Bars grow upward from bottom of container
                    const barBottom = containerY + containerH - 4;
                    barH = h * 2;
                    barY = barBottom - barH;
                } else {
                    // Bars grow from center (symmetric)
                    barH = h * 2;
                    barY = centerY - h;
                }

                // Glow effect
                if (db.glowFromTheme) {
                    cr.setSourceRGBA(r, g, b, alpha * 0.3);
                    _roundedRect(cr, x - 1, barY - 1, effectiveBarWidth + 2, barH + 2, barRadius + 1);
                    cr.fill();
                }

                cr.setSourceRGBA(r, g, b, alpha);
                _roundedRect(cr, x, barY, effectiveBarWidth, barH, barRadius);
                cr.fill();
            }

            // Scanlines hint from layers
            const layers = design.layers || [];
            for (const layer of layers) {
                if (layer.type === 'scanlines') {
                    const [slr, slg, slb] = layer.color || [0, 0, 0];
                    const sla = layer.alpha || 0.15;
                    const lineSpacing = layer.lineSpacing || 3;
                    cr.setSourceRGBA(slr / 255, slg / 255, slb / 255, sla);
                    for (let ly = containerY + 2; ly < containerY + containerH - 2; ly += lineSpacing + 1) {
                        cr.rectangle(containerX + 2, ly, containerW - 4, 1);
                        cr.fill();
                    }
                }
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

        // Modifier slider (visual effects intensity)
        const modifierRow = new Adw.ActionRow({
            title: 'Modifier',
            subtitle: 'Shadow trail effect intensity',
        });

        const modAdj = new Gtk.Adjustment({
            lower: 0,
            upper: 100,
            step_increment: 1,
            page_increment: 10,
            value: settings.get_int('wave-modifier'),
        });

        const modScale = new Gtk.Scale({
            adjustment: modAdj,
            orientation: Gtk.Orientation.HORIZONTAL,
            draw_value: false,
            hexpand: true,
            valign: Gtk.Align.CENTER,
            width_request: 200,
        });

        modScale.connect('value-changed', () => {
            settings.set_int('wave-modifier', Math.round(modScale.get_value()));
            previewArea.queue_draw();
        });

        modifierRow.add_suffix(modScale);
        themeGroup.add(modifierRow);

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
        sizeGroup.add(waveTypeRow);

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
            settings.connect('changed::wave-design', () => {
                const idx = DESIGN_IDS.indexOf(settings.get_string('wave-design'));
                if (idx >= 0) designRow.set_selected(idx);
                previewArea.queue_draw();
            })
        );

        settingsHandlerIds.push(
            settings.connect('changed::wave-position', () => {
                currentPosition = settings.get_string('wave-position');
                positionArea.queue_draw();
            })
        );

        settingsHandlerIds.push(
            settings.connect('changed::wave-modifier', () => {
                modAdj.set_value(settings.get_int('wave-modifier'));
                previewArea.queue_draw();
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

// Helper: blend two {r,g,b} colors
function _blendColor(a, b, t) {
    return {
        r: Math.round(a.r + (b.r - a.r) * t),
        g: Math.round(a.g + (b.g - a.g) * t),
        b: Math.round(a.b + (b.b - a.b) * t),
    };
}

// Alias for layerPainter's rounded rect (used in bar rendering and position preview)
const _roundedRect = LayerPainter.cairoRoundedRect;

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
