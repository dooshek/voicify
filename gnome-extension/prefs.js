'use strict';

import Adw from 'gi://Adw';
import Gtk from 'gi://Gtk';

import { ExtensionPreferences } from 'resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js';

export default class VoicifyPreferences extends ExtensionPreferences {
    fillPreferencesWindow(window) {
        const page = new Adw.PreferencesPage({
            title: 'General',
            icon_name: 'audio-input-microphone-symbolic',
        });
        window.add(page);

        const group = new Adw.PreferencesGroup({
            title: 'Keyboard Shortcuts',
            description: 'Configure keyboard shortcuts for Voicify',
        });
        page.add(group);

        const shortcutRow = new Adw.ActionRow({
            title: 'Voice Recording Shortcut',
            subtitle: 'Current: Ctrl+Win+V (hardcoded for now)',
        });

        const infoLabel = new Gtk.Label({
            label: 'Shortcut configuration coming soon...',
            css_classes: ['dim-label'],
        });

        shortcutRow.add_suffix(infoLabel);
        group.add(shortcutRow);

        // Add info about current functionality
        const infoGroup = new Adw.PreferencesGroup({
            title: 'Current Features',
        });
        page.add(infoGroup);

        const featuresRow = new Adw.ActionRow({
            title: 'Available Features',
            subtitle: '• Global shortcut: Ctrl+Win+V\n• Voice recording simulation\n• Text injection via clipboard',
        });
        infoGroup.add(featuresRow);
    }
}