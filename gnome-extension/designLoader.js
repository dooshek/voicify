'use strict';

import Gio from 'gi://Gio';
import GLib from 'gi://GLib';

const DESIGN_DEFAULTS = {
    name: 'Unnamed',
    sortOrder: 99,
    defaultTheme: null,
    uploadHueShift: 120,
    container: {
        borderRadius: 20,
        bgColor: [0, 0, 0],
        bgOpacity: 0.25,
        blur: null,
    },
    bars: {
        borderRadius: 2,
        shadow: null,
        opacityMode: 'gradient',
        opacityMin: 80,
        opacityMax: 255,
        opacityUniform: 255,
        pivotY: 0.5,
        scaleMin: 0.04,
        scaleMax: 1.0,
        glowFromTheme: false,
        glowRadius: 0,
        glowAlpha: 0,
        highlight: false,
        widthAdjust: 0,
        colorMute: 0,
        colorOverride: null,
    },
    layers: [],
};

function _deepMerge(defaults, overrides) {
    const result = {};
    for (const key of Object.keys(defaults)) {
        const defVal = defaults[key];
        const overVal = overrides[key];

        if (overVal === undefined) {
            result[key] = defVal;
        } else if (defVal !== null && typeof defVal === 'object' && !Array.isArray(defVal)
                   && overVal !== null && typeof overVal === 'object' && !Array.isArray(overVal)) {
            result[key] = _deepMerge(defVal, overVal);
        } else {
            result[key] = overVal;
        }
    }
    // Copy keys present in overrides but not in defaults (e.g. layer items)
    for (const key of Object.keys(overrides)) {
        if (!(key in defaults)) {
            result[key] = overrides[key];
        }
    }
    return result;
}

const FALLBACK_GNOME = {
    name: 'Gnome',
    sortOrder: 1,
    container: {
        borderRadius: 30,
        bgColor: [54, 54, 58],
        bgOpacity: 1.0,
        defaultWidth: 104,
        defaultHeight: 69,
    },
    bars: {
        borderRadius: 3,
        shadow: null,
        opacityMode: 'uniform',
        opacityMin: 80,
        opacityMax: 255,
        opacityUniform: 220,
        pivotY: 0.5,
        scaleMin: 0.04,
        scaleMax: 1.0,
        glowFromTheme: false,
        glowRadius: 0,
        glowAlpha: 0,
        highlight: false,
        widthAdjust: 0,
        colorMute: 0.15,
        colorOverride: null,
    },
    layers: [],
};

/**
 * Load all design JSON files from the designs/ directory.
 * @param {string} extensionPath - Path to the extension directory
 * @returns {Map<string, object>} Map of design ID -> design object, sorted by sortOrder
 */
export function loadDesigns(extensionPath) {
    const designs = new Map();
    const designsDir = GLib.build_filenamev([extensionPath, 'designs']);
    const dir = Gio.File.new_for_path(designsDir);

    if (!dir.query_exists(null)) {
        console.debug('Voicify: designs/ directory not found, using fallback');
        designs.set('gnome', FALLBACK_GNOME);
        return designs;
    }

    try {
        const enumerator = dir.enumerate_children(
            'standard::name,standard::type',
            Gio.FileQueryInfoFlags.NONE,
            null
        );

        let fileInfo;
        while ((fileInfo = enumerator.next_file(null)) !== null) {
            const name = fileInfo.get_name();
            if (!name.endsWith('.json')) continue;

            const id = name.slice(0, -5); // strip .json
            const filePath = GLib.build_filenamev([designsDir, name]);

            try {
                const [ok, contents] = GLib.file_get_contents(filePath);
                if (!ok) continue;

                const decoder = new TextDecoder('utf-8');
                const json = JSON.parse(decoder.decode(contents));
                const merged = _deepMerge(DESIGN_DEFAULTS, json);
                designs.set(id, merged);
            } catch (e) {
                console.error(`Voicify: failed to load design ${name}: ${e.message}`);
            }
        }
        enumerator.close(null);
    } catch (e) {
        console.error(`Voicify: failed to enumerate designs/: ${e.message}`);
    }

    if (designs.size === 0) {
        console.debug('Voicify: no designs loaded, using fallback');
        designs.set('gnome', FALLBACK_GNOME);
        return designs;
    }

    // Sort by sortOrder
    const sorted = new Map(
        [...designs.entries()].sort((a, b) => (a[1].sortOrder || 99) - (b[1].sortOrder || 99))
    );

    return sorted;
}

/**
 * Get sorted array of design IDs.
 * @param {Map<string, object>} designs - Map from loadDesigns()
 * @returns {string[]} Array of design IDs in sort order
 */
export function getDesignIds(designs) {
    return [...designs.keys()];
}

/**
 * Apply user overrides to a design.
 * @param {object} design - Base design object
 * @param {string} overridesJson - JSON string from design-overrides setting
 * @param {string} designId - Current design ID
 * @returns {object} Design with overrides applied (deep clone)
 */
export function applyOverrides(design, overridesJson, designId) {
    let allOverrides;
    try {
        allOverrides = JSON.parse(overridesJson || '{}');
    } catch (e) {
        return design;
    }
    const o = allOverrides[designId];
    if (!o) return design;

    const result = JSON.parse(JSON.stringify(design));
    if (o.container) Object.assign(result.container, o.container);
    if (o.bars) Object.assign(result.bars, o.bars);
    if (o.layers) {
        result.layers = result.layers.map(layer => {
            const lo = o.layers[layer.type];
            if (!lo) return layer;
            if (lo.enabled === false) return null;
            return { ...layer, ...lo };
        }).filter(Boolean);
    }
    return result;
}
