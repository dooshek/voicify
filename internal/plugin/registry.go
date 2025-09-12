package plugin

// RegisterAllPlugins registers all built-in plugins with the manager
func RegisterAllPlugins(manager *Manager) error {
	// Register Linear plugin
	linearPlugin := NewPluginAdapter(NewLinearPlugin())
	if err := manager.RegisterPlugin(linearPlugin); err != nil {
		return err
	}

	// Register VSCode plugin
	vscodePlugin := NewPluginAdapter(NewVSCodePlugin())
	if err := manager.RegisterPlugin(vscodePlugin); err != nil {
		return err
	}

	// Register Default plugin (always last)
	defaultPlugin := NewPluginAdapter(NewDefaultPlugin())
	if err := manager.RegisterPlugin(defaultPlugin); err != nil {
		return err
	}

	return nil
}
