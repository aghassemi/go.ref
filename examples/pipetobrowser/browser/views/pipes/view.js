import { View } from 'libs/mvc/view'

/*
 * View representing a collection of pipes displayed in tabs.
 * this view manages the tabs and the empty message when no pipes available
 * @class
 * @extends {View}
 */
export class PipesView extends View {
  constructor() {
    var el = document.createElement('p2b-pipes');
    super(el);
  }

 /*
  * Adds the given view as a new pipe viewer tab
  * @param {string} key A string key identifier for the tab.
  * @param {string} name A short name for the tab that will be displayed as
  * the tab title
  * @param {View} view View to show inside the tab.
  * @param {function} onClose Optional onClose callback.
  */
  addTab(key, name, view, onClose) {
    this.element.addTab(key, name, view.element, onClose);
  }

  /*
   * Adds a new toolbar action for the tab's toolbar
   * @param {string} key Key of the tab to add action to
   * @param icon {string} icon key for the action
   * @param onClick {function} event handler for the action
   */
  addToolbarAction(tabKey, icon, onClick) {
    this.element.addToolbarAction(tabKey, icon, onClick);
  }

 /*
  * Replaces the content of the tab identified via key by the new view.
  * @param {string} key A string key identifier for the tab.
  * @param {string} newName New name for the tab
  * @param {View} view View to replace the current tab content
  */
  replaceTabView(key, newName, newView) {
    this.element.replaceTabContent(key, newName, newView.element);
  }
}
