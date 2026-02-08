'use strict';

// Cairo rounded rectangle helper
export function cairoRoundedRect(cr, x, y, w, h, r) {
    r = Math.min(r, w / 2, h / 2);
    cr.newSubPath();
    cr.arc(x + w - r, y + r, r, -Math.PI / 2, 0);
    cr.arc(x + w - r, y + h - r, r, 0, Math.PI / 2);
    cr.arc(x + r, y + h - r, r, Math.PI / 2, Math.PI);
    cr.arc(x + r, y + r, r, Math.PI, 3 * Math.PI / 2);
    cr.closePath();
}

// Draw all canvas layers at given offset (used by prefs.js preview)
export function drawAllCanvasLayersAt(cr, design, theme, x, y, w, h, skipShadow = false) {
    const dc = design.container;
    const layers = design.layers || [];
    const radius = Math.min(h / 2, dc.borderRadius);

    // Pass 1: shadow (behind everything) - skip if handled by CSS box-shadow
    if (!skipShadow) {
        for (const layer of layers) {
            if (layer.type === 'shadow')
                _drawShadow(cr, layer, x, y, w, h, radius);
        }
    }

    // Background fill (the body - behind border and highlights)
    const [bgR, bgG, bgB] = dc.bgColor || [0, 0, 0];
    const bgA = dc.bgOpacity ?? 0.25;
    cr.setSourceRGBA(bgR / 255, bgG / 255, bgB / 255, bgA);
    cairoRoundedRect(cr, x, y, w, h, radius);
    cr.fill();

    // Pass 2: surface effects (on top of body)
    for (const layer of layers) {
        switch (layer.type) {
            case 'border':
                _drawBorder(cr, layer, theme, x, y, w, h, radius);
                break;
            case 'innerHighlight':
                _drawInnerHighlight(cr, layer, x, y, w, h, radius);
                break;
            case 'specularHighlight':
                _drawSpecularHighlight(cr, layer, x, y, w, h, radius);
                break;
            case 'innerShadow':
                _drawInnerShadow(cr, layer, x, y, w, h, radius);
                break;
        }
    }
}

// Draw all canvas layers at (0,0), optionally skipping shadow (handled by CSS)
export function drawAllCanvasLayers(cr, design, theme, w, h, skipShadow = false) {
    drawAllCanvasLayersAt(cr, design, theme, 0, 0, w, h, skipShadow);
}

function _drawShadow(cr, layer, x, y, w, h, radius) {
    const [sr, sg, sb] = layer.color || [0, 0, 0];
    const alpha = layer.alpha || 0.3;
    const blur = layer.blur || 8;
    const ox = layer.x || 0;
    const oy = layer.y || 0;

    // Fewer, thicker rings: avoids sub-pixel aliasing artifacts (min ~2px width)
    const steps = Math.max(4, Math.ceil(blur / 3));
    const ringWidth = blur / steps;

    for (let i = 0; i < steps; i++) {
        const t = (i + 0.5) / steps; // midpoint fraction (0=body edge, 1=outer edge)
        const spread = blur * t;

        // Gaussian-like falloff
        const a = alpha * Math.exp(-t * t * 4);
        if (a < 0.001) continue;

        cr.setLineWidth(ringWidth);
        cr.setSourceRGBA(sr / 255, sg / 255, sb / 255, a);
        cairoRoundedRect(cr, x + ox - spread, y + oy - spread,
            w + spread * 2, h + spread * 2, radius + spread);
        cr.stroke();
    }
}

function _drawBorder(cr, layer, theme, x, y, w, h, radius) {
    const bw = layer.width || 1;
    const alpha = layer.alpha || 0.3;

    let r, g, b;
    if (layer.source === 'theme') {
        r = theme.center.r / 255;
        g = theme.center.g / 255;
        b = theme.center.b / 255;
    } else {
        const [cr2, cg, cb] = layer.color || [255, 255, 255];
        r = cr2 / 255;
        g = cg / 255;
        b = cb / 255;
    }

    cr.setLineWidth(bw);

    if (layer.gradient) {
        // Vertical gradient from alpha (top) to alphaBottom (bottom)
        const alphaBottom = layer.alphaBottom ?? alpha * 0.2;
        const steps = 12;
        for (let s = 0; s < steps; s++) {
            const t = s / (steps - 1);
            const a = alpha + (alphaBottom - alpha) * t;
            const stripY = y + (h * s / steps);
            const stripH = h / steps + 1;
            cr.save();
            cr.rectangle(x - bw, stripY, w + 2 * bw, stripH);
            cr.clip();
            cr.setSourceRGBA(r, g, b, a);
            cairoRoundedRect(cr, x + bw / 2, y + bw / 2, w - bw, h - bw, Math.max(0, radius - bw / 2));
            cr.stroke();
            cr.restore();
        }
        return;
    } else {
        cr.setSourceRGBA(r, g, b, alpha);
    }

    cairoRoundedRect(cr, x + bw / 2, y + bw / 2, w - bw, h - bw, Math.max(0, radius - bw / 2));
    cr.stroke();
}

function _drawInnerHighlight(cr, layer, x, y, w, h, radius) {
    const alpha = layer.alpha || 0.08;
    const heightFrac = layer.height || 0.35;

    cr.save();
    cairoRoundedRect(cr, x, y, w, h, radius);
    cr.clip();

    // Smooth gradient from alpha at top to transparent at heightFrac
    const highlightH = h * heightFrac;
    const steps = 8;
    for (let i = 0; i < steps; i++) {
        const t = i / (steps - 1);
        const a = alpha * (1 - t);
        const stripH = highlightH / steps + 1;
        cr.setSourceRGBA(1, 1, 1, a);
        cr.rectangle(x, y + (highlightH * i / steps), w, stripH);
        cr.fill();
    }
    cr.restore();
}

function _drawSpecularHighlight(cr, layer, x, y, w, h, radius) {
    const alpha = layer.alpha || 0.05;
    const fx = layer.x ?? 0.5;
    const fy = layer.y ?? 0.1;
    const fw = layer.width ?? 0.7;
    const fh = layer.height ?? 0.25;

    cr.save();
    cairoRoundedRect(cr, x, y, w, h, radius);
    cr.clip();

    const sx = x + (1 - fw) * w * fx;
    const sy = y + h * fy;
    const sw = w * fw;
    const sh = h * fh;

    cr.setSourceRGBA(1, 1, 1, alpha);
    cr.save();
    cr.translate(sx + sw / 2, sy + sh / 2);
    cr.scale(sw / 2, sh / 2);
    cr.arc(0, 0, 1, 0, 2 * Math.PI);
    cr.restore();
    cr.fill();
    cr.restore();
}

function _drawInnerShadow(cr, layer, x, y, w, h, radius) {
    const [sr, sg, sb] = layer.color || [0, 0, 0];
    const alpha = layer.alpha || 0.06;
    const blur = layer.blur || 8;

    cr.save();
    cairoRoundedRect(cr, x, y, w, h, radius);
    cr.clip();

    // Inner shadow that follows the rounded shape (inset stroke)
    const steps = Math.max(4, Math.ceil(blur / 2));
    for (let i = 0; i < steps; i++) {
        const t = i / steps;
        const a = alpha * (1 - t);
        const inset = (blur / steps) * i;
        cr.setLineWidth(2);
        cr.setSourceRGBA(sr / 255, sg / 255, sb / 255, a);
        cairoRoundedRect(cr, x + inset, y + inset, w - inset * 2, h - inset * 2,
            Math.max(0, radius - inset));
        cr.stroke();
    }
    cr.restore();
}
