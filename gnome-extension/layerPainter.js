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
export function drawAllCanvasLayersAt(cr, design, theme, x, y, w, h) {
    const dc = design.container;
    const layers = design.layers || [];
    const radius = Math.min(h / 2, dc.borderRadius);

    for (const layer of layers) {
        switch (layer.type) {
            case 'shadow':
                _drawShadow(cr, layer, x, y, w, h, radius);
                break;
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
            // pixelGrid and scanlines are widget layers, not drawn here
        }
    }

    // Background fill
    const [bgR, bgG, bgB] = dc.bgColor || [0, 0, 0];
    const bgA = dc.bgOpacity ?? 0.25;
    cr.setSourceRGBA(bgR / 255, bgG / 255, bgB / 255, bgA);
    cairoRoundedRect(cr, x, y, w, h, radius);
    cr.fill();
}

// Draw all canvas layers at (0,0)
export function drawAllCanvasLayers(cr, design, theme, w, h) {
    drawAllCanvasLayersAt(cr, design, theme, 0, 0, w, h);
}

function _drawShadow(cr, layer, x, y, w, h, radius) {
    const [sr, sg, sb] = layer.color || [0, 0, 0];
    const alpha = layer.alpha || 0.3;
    const blur = layer.blur || 8;
    const ox = layer.x || 0;
    const oy = layer.y || 0;

    // Approximate shadow with multiple fading rectangles
    const steps = Math.max(3, Math.ceil(blur / 2));
    for (let i = steps; i >= 1; i--) {
        const spread = (blur / steps) * i;
        const a = alpha * (1 - i / (steps + 1)) * 0.5;
        cr.setSourceRGBA(sr / 255, sg / 255, sb / 255, a);
        cairoRoundedRect(cr, x + ox - spread, y + oy - spread,
            w + spread * 2, h + spread * 2, radius + spread);
        cr.fill();
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
    cr.setSourceRGBA(r, g, b, alpha);
    cairoRoundedRect(cr, x + bw / 2, y + bw / 2, w - bw, h - bw, Math.max(0, radius - bw / 2));
    cr.stroke();
}

function _drawInnerHighlight(cr, layer, x, y, w, h, radius) {
    const alpha = layer.alpha || 0.08;
    const heightFrac = layer.height || 0.35;

    cr.save();
    cairoRoundedRect(cr, x, y, w, h, radius);
    cr.clip();

    const highlightH = h * heightFrac;
    cr.setSourceRGBA(1, 1, 1, alpha);
    cr.rectangle(x, y, w, highlightH);
    cr.fill();
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

    // Bottom inner shadow
    const steps = Math.max(2, Math.ceil(blur / 3));
    for (let i = 0; i < steps; i++) {
        const a = alpha * (1 - i / steps);
        const inset = (blur / steps) * i;
        cr.setSourceRGBA(sr / 255, sg / 255, sb / 255, a);
        cr.rectangle(x, y + h - inset - 2, w, 2);
        cr.fill();
    }
    cr.restore();
}
