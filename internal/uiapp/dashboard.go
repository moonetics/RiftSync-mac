package uiapp

import (
	"encoding/base64"

	"riftsync/assets"
	"riftsync/internal/serverapp"
)

func htmlDocument() string {
	brandIcon := base64.StdEncoding.EncodeToString(assets.RiftSyncIconPNG)
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>RiftSync</title>
<style>
:root {
  color-scheme:dark;
  --space-1:4px;--space-2:8px;--space-3:12px;--space-4:16px;--space-6:24px;--space-8:32px;
  --radius-sm:8px;--radius-md:12px;--radius-lg:16px;--blur:14px;
  --bg:#0b1118;--shell:#151d27;--glass:#17212c;--surface:#1b2632;--surface-hi:#253341;
  --well:#101821;--line:#405064;--line-soft:#2d3c4d;--ink:#f2f6fb;--muted:#aeb9c8;
  --cyan:#68d7d0;--blue:#829bf0;--green:#66dc93;--amber:#f0c36d;--red:#f0818b;
  --shadow:0 10px 28px #02060b66;--rail:64px;--side:244px;
}
*{box-sizing:border-box;scrollbar-width:none;-ms-overflow-style:none}
*::-webkit-scrollbar{display:none;width:0;height:0}
html,body{width:100%;height:100%;margin:0;overflow:hidden;background:var(--bg);color:var(--ink);font:14px/1.5 "Segoe UI",system-ui,sans-serif}
body{background-image:radial-gradient(circle at 18% 12%,#163447 0,transparent 34%),radial-gradient(circle at 88% 80%,#202a4a 0,transparent 30%)}
button,input,textarea{font:inherit}
button{min-height:40px;border:1px solid var(--line);border-radius:var(--radius-sm);padding:8px 14px;color:var(--ink);background:var(--surface-hi);box-shadow:none;cursor:pointer;font-weight:650;transition:background-color .16s ease,border-color .16s ease,color .16s ease,opacity .16s ease}
button:hover{border-color:#647991;background:#2d3d4d}button:active{background:#1f2b37}button:disabled{opacity:.45;cursor:not-allowed}
button.primary{color:#071716;border-color:var(--cyan);background:var(--cyan)}button.primary:hover{background:#87e5df}
button.danger{color:#ffe9eb;border-color:#96545c;background:#512d34}button.danger:hover{background:#63363e}
button.icon-button{width:40px;min-width:40px;padding:0}
button:focus-visible,input:focus-visible,textarea:focus-visible,.project-button:focus-visible,[tabindex]:focus-visible{outline:3px solid var(--cyan);outline-offset:2px}
input,textarea{width:100%;min-width:0;border:1px solid var(--line);border-radius:var(--radius-sm);background:var(--well);color:var(--ink);box-shadow:none;padding:10px 12px}
input:disabled,textarea:disabled{opacity:.6}textarea{resize:none;font:13px/1.55 Consolas,"Cascadia Mono",monospace}
.app{display:grid;grid-template-columns:var(--rail) minmax(0,1fr);grid-template-rows:64px minmax(0,1fr);width:100%;height:100%;overflow:hidden}
.app.sidebar-pinned{grid-template-columns:var(--side) minmax(0,1fr)}
.glass{background:var(--shell);border:1px solid var(--line-soft);box-shadow:var(--shadow)}
@supports ((backdrop-filter:blur(1px)) or (-webkit-backdrop-filter:blur(1px))){.glass,.panel,.modal,.sidebar{background:rgba(23,33,44,.84);backdrop-filter:blur(var(--blur));-webkit-backdrop-filter:blur(var(--blur))}}
.titlebar{grid-column:1/-1;display:flex;align-items:center;gap:var(--space-3);min-width:0;padding:var(--space-2) var(--space-3);border-width:0 0 1px;background:var(--shell);user-select:none;z-index:50}
.brand{display:flex;align-items:center;gap:var(--space-2);min-width:190px}.brand-logo{display:block;width:32px;height:32px;flex:0 0 32px;object-fit:contain}.brand strong{font-size:18px;letter-spacing:.2px}.brand-version{display:inline-flex;align-items:center;margin-left:6px;padding:1px 6px;border:1px solid var(--line);border-radius:4px;color:var(--cyan);font:700 10px/1.5 Consolas,monospace;vertical-align:2px}.brand small{display:block;color:var(--muted);font-size:10px;letter-spacing:.08em}
.title-context{display:flex;align-items:center;justify-content:center;gap:var(--space-2);min-width:0;flex:1}.title-name{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-weight:750}.address{color:var(--muted);font:12px/1.5 Consolas,monospace}.state-text{color:var(--muted);font-size:12px;font-weight:700}
.lamp{width:10px;height:10px;flex:0 0 10px;border-radius:50%;background:#8d99a8}.lamp.stopped{background:#8d99a8}.lamp.syncing{background:var(--cyan)}.lamp.running{background:var(--green)}.lamp.starting{background:var(--amber)}.lamp.error{background:var(--red)}
.instance-actions,.window-actions{display:flex;align-items:center;gap:var(--space-2);flex-shrink:0}
.sidebar-wrap{position:relative;z-index:40;grid-row:2;min-width:0}.sidebar{position:absolute;inset:0 auto 0 0;width:var(--rail);display:flex;flex-direction:column;overflow:hidden;border-width:0 1px 0 0;background:var(--shell);transition:width .16s ease}
.sidebar-wrap:hover .sidebar,.sidebar-wrap:focus-within .sidebar,.sidebar-pinned .sidebar{width:var(--side)}
.sidebar-head{display:flex;align-items:center;gap:var(--space-3);height:56px;padding:var(--space-2) var(--space-3);white-space:nowrap;border-bottom:1px solid var(--line-soft)}.sidebar-head strong{opacity:0;transition:opacity .12s}.sidebar-wrap:hover .sidebar-head strong,.sidebar-wrap:focus-within .sidebar-head strong,.sidebar-pinned .sidebar-head strong{opacity:1}
.project-list{flex:1;min-height:0;overflow-y:auto;overflow-x:hidden;padding:var(--space-2)}
.project-button{position:relative;display:grid;grid-template-columns:40px 0;justify-content:center;align-items:center;gap:0;width:100%;min-width:0;margin-bottom:var(--space-2);padding:4px;border:1px solid transparent;border-radius:var(--radius-md);background:transparent;color:var(--ink);text-align:left;cursor:pointer}
.sidebar-wrap:hover .project-button,.sidebar-wrap:focus-within .project-button,.sidebar-pinned .project-button{grid-template-columns:40px minmax(0,1fr);justify-content:stretch;gap:var(--space-2)}
.project-button:hover{background:#253241;border-color:var(--line-soft)}.project-button.active{background:#293847;border-color:#567086}.project-button.active:before{content:"";position:absolute;left:-5px;width:3px;height:24px;border-radius:3px;background:var(--cyan)}
.avatar{position:relative;display:grid;place-items:center;width:40px;height:40px;border-radius:var(--radius-sm);background:var(--avatar,#49607d);font-weight:850}.avatar .lamp{position:absolute;right:-2px;bottom:-2px;border:2px solid var(--surface);width:11px;height:11px}
.project-copy{min-width:0;opacity:0;transition:opacity .12s}.sidebar-wrap:hover .project-copy,.sidebar-wrap:focus-within .project-copy,.sidebar-pinned .project-copy{opacity:1}.project-copy strong,.project-copy span{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.project-copy span{color:var(--muted);font-size:12px}
.sidebar-foot{display:grid;gap:var(--space-2);padding:var(--space-2)}.side-action{display:grid;grid-template-columns:46px minmax(0,1fr);align-items:center;gap:0;width:100%;padding:0;overflow:hidden}.side-icon{display:grid;place-items:center;width:46px;height:38px;line-height:1}.side-icon svg{display:block;width:16px;height:16px}.side-action .action-label{min-width:0;padding-right:var(--space-2);opacity:0;white-space:nowrap;text-align:center}.sidebar-wrap:hover .action-label,.sidebar-wrap:focus-within .action-label,.sidebar-pinned .action-label{opacity:1}
.sidebar-wrap.force-collapsed .sidebar{width:var(--rail);transition:none}.sidebar-wrap.force-collapsed .project-button{grid-template-columns:40px 0;justify-content:center;gap:0}.sidebar-wrap.force-collapsed .sidebar-head strong,.sidebar-wrap.force-collapsed .project-copy,.sidebar-wrap.force-collapsed .action-label{opacity:0}
.workspace{grid-column:2;grid-row:2;min-width:0;min-height:0;display:grid;grid-template-rows:56px minmax(0,1fr);overflow:hidden}.toolbar{display:flex;align-items:end;min-width:0;padding:var(--space-2) var(--space-4) 0;border-bottom:1px solid var(--line-soft);background:#111a23cc}.tabs{display:flex;gap:var(--space-1);min-width:0;overflow-x:auto}.tab{min-width:96px;border-color:transparent;border-radius:var(--radius-sm) var(--radius-sm) 0 0;background:transparent;color:var(--muted)}.tab:hover{background:#253241}.tab.active{color:var(--cyan);background:#1d2935;border-color:var(--line-soft);border-bottom-color:transparent}
.content{min-width:0;min-height:0;overflow:hidden;padding:var(--space-4)}.panel-page{display:none;width:100%;height:100%;min-width:0;min-height:0;overflow:hidden}.panel-page.active{display:block}
.panel{min-width:0;min-height:0;border:1px solid var(--line-soft);border-radius:var(--radius-md);background:var(--glass);box-shadow:var(--shadow);padding:var(--space-4);overflow:hidden}.panel h2{margin:0 0 var(--space-1);font-size:16px}.label{color:var(--muted);font-size:11px;font-weight:800;text-transform:uppercase;letter-spacing:.08em}.hint{color:var(--muted);font-size:12px;overflow-wrap:anywhere}.value{margin-top:var(--space-1);font-size:22px;font-weight:800;overflow-wrap:anywhere}
.overview{display:grid;grid-template-rows:auto auto minmax(0,1fr);gap:var(--space-3);height:100%;min-height:0;overflow:auto}.summary-card{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:start;gap:var(--space-4)}.project-name{margin:var(--space-2) 0 var(--space-1);font-size:24px;font-weight:800;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.path{font:12px/1.5 Consolas,monospace;color:#d0d8e4;overflow-wrap:anywhere}.summary-state{display:grid;grid-template-columns:10px auto;align-items:center;justify-self:start;gap:var(--space-2);min-height:46px;padding:6px 10px;border:1px solid var(--line-soft);border-radius:var(--radius-sm);background:var(--surface);white-space:nowrap}.summary-state strong{display:block;font-size:14px;line-height:1.2}.summary-state .hint{margin-top:2px;line-height:1.2}.summary-activity{grid-column:1/-1;display:grid;grid-template-columns:96px minmax(0,1fr);gap:var(--space-3);padding-top:var(--space-3);border-top:1px solid var(--line-soft)}.activity-progress{position:relative;width:100%;height:8px;margin-top:8px;overflow:hidden;border:1px solid var(--line-soft);border-radius:4px;background:var(--well)}.activity-progress-fill{display:block;width:0;height:100%;border-radius:3px;background:var(--cyan);transition:width .16s ease}.activity-progress.indeterminate .activity-progress-fill{width:34%;animation:activity-slide 1.1s linear infinite}@keyframes activity-slide{from{transform:translateX(-105%)}to{transform:translateX(300%)}}
.metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:var(--space-3)}.metric{min-width:0;padding:var(--space-4);border:1px solid var(--line-soft);border-radius:var(--radius-md);background:var(--surface);box-shadow:none}.button-row{display:flex;flex-wrap:wrap;gap:var(--space-2);margin-top:var(--space-4)}
.disclosure{width:100%;align-self:stretch}.disclosure-trigger{display:flex;align-items:center;justify-content:space-between;width:100%;padding:0;border:0;background:transparent;text-align:left}.disclosure-trigger>span:first-child{display:flex;align-items:baseline;flex-wrap:wrap;gap:var(--space-2);min-width:0}.disclosure-trigger:hover{background:transparent;color:var(--cyan)}.disclosure-trigger:after{content:"+";font-size:20px}.disclosure-trigger[aria-expanded="true"]:after{content:"−"}.disclosure-body{padding-top:var(--space-4)}.disclosure-body[hidden]{display:none}.health-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:var(--space-2)}.health-cell{min-width:0;border:1px solid var(--line-soft);border-radius:var(--radius-sm);background:var(--well);padding:var(--space-3)}.health-cell .value{font-size:18px}.truncate{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.error{color:#ffb6bc}.activity-notice{display:flex;align-items:center;gap:var(--space-1);min-width:0;margin-top:6px;font-size:12px;overflow-wrap:anywhere;cursor:default}.activity-notice.success{color:#9cebb9}.activity-notice.error{color:#ffb6bc}
.history-shell{display:grid;grid-template-columns:minmax(220px,.36fr) minmax(0,1fr);gap:var(--space-3);height:100%;min-height:0}.history-list,.history-detail{min-height:0;overflow:auto}.history-item{width:100%;min-width:0;text-align:left;margin-bottom:var(--space-2);background:var(--surface)}.history-item span{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.history-item small{color:var(--muted)}.change-row{min-width:0;padding:var(--space-3) 0;border-bottom:1px solid var(--line-soft)}.change-row div{color:var(--muted);font-size:12px;overflow-wrap:anywhere}
.exec-shell{display:grid;grid-template-columns:minmax(330px,.9fr) minmax(360px,1.1fr);gap:var(--space-3);height:100%;min-height:0}.exec-editor{display:grid;grid-template-rows:auto auto minmax(120px,1fr) auto;gap:var(--space-3)}.exec-result{display:grid;grid-template-rows:auto minmax(100px,1fr) auto minmax(118px,.72fr);gap:var(--space-2)}.token-row,.input-button{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:var(--space-2)}.exec-toolbar{display:grid;grid-template-columns:100px minmax(0,1fr) auto auto;align-items:end;gap:var(--space-2)}.output,.code-view{min-width:0;min-height:0;margin:0;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;border:1px solid var(--line-soft);border-radius:var(--radius-sm);background:#0a1118;color:#c8f5df;padding:var(--space-3);font:13px/1.55 Consolas,monospace}
.recent-head{display:flex;align-items:center;justify-content:space-between;gap:var(--space-2)}.exec-history{min-width:0;min-height:0;overflow-y:auto;overflow-x:hidden;border:1px solid var(--line-soft);border-radius:var(--radius-sm);background:var(--well);padding:var(--space-1)}.exec-history-item{display:grid;grid-template-columns:minmax(0,1fr) auto auto;align-items:center;gap:var(--space-2);min-width:0;padding:var(--space-2);border-bottom:1px solid var(--line-soft)}.exec-history-item:last-child{border:0}.exec-history-item button{min-height:36px}.exec-history-copy{min-width:0}.exec-history-title,.exec-history-meta{display:block;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.exec-history-title{font-weight:750}.exec-history-meta{color:var(--muted);font-size:12px}
.config-shell{display:grid;grid-template-columns:minmax(0,1fr) minmax(280px,.72fr);gap:var(--space-3);height:100%;min-height:0;overflow:auto}.form-panel{overflow:auto}.field{min-width:0;margin-top:var(--space-3)}.field label{display:block;margin-bottom:var(--space-1)}.form-grid{display:grid;grid-template-columns:minmax(0,1fr) 140px;gap:var(--space-3)}.toggle-row{display:flex;justify-content:space-between;align-items:center;gap:var(--space-3);padding:var(--space-3);border:1px solid var(--line-soft);border-radius:var(--radius-sm);background:var(--well)}.toggle-row>span{display:flex;align-items:baseline;flex-wrap:wrap;gap:var(--space-2);min-width:0}.toggle-row input{width:20px;height:20px}.config-side{display:flex;flex-direction:column;align-items:stretch;gap:var(--space-3)}.config-side .panel{width:100%;overflow:visible}.danger-zone{border-color:#70434a}
.protection-shell{display:grid;grid-template-columns:minmax(320px,1.15fr) minmax(300px,.85fr);gap:var(--space-3);height:100%;min-height:0}
.explorer-panel{display:flex;flex-direction:column;gap:var(--space-2);height:100%;min-height:0;overflow:hidden}
.explorer-header{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:var(--space-2);flex-shrink:0}
.explorer-search-bar{display:flex;gap:var(--space-2);align-items:center;flex-shrink:0}
.explorer-tree-wrap{flex:1;min-height:0;overflow-y:auto;overflow-x:hidden;border:1px solid var(--line-soft);border-radius:var(--radius-sm);background:#0f1620;padding:var(--space-1)}
.explorer-tree-wrap::-webkit-scrollbar, .sub-panel::-webkit-scrollbar{width:6px;height:6px}
.explorer-tree-wrap::-webkit-scrollbar-track, .sub-panel::-webkit-scrollbar-track{background:#0a1017}
.explorer-tree-wrap::-webkit-scrollbar-thumb, .sub-panel::-webkit-scrollbar-thumb{background:#2b3d52;border-radius:3px}
.explorer-tree-wrap::-webkit-scrollbar-thumb:hover, .sub-panel::-webkit-scrollbar-thumb:hover{background:var(--cyan)}
.tree-row{display:flex;align-items:center;height:24px;padding:1px 6px;border-radius:4px;user-select:none;cursor:pointer;font-size:12.5px;white-space:nowrap}
.tree-row:hover{background:#1c2734}
.tree-row.selected{background:#233345;color:var(--cyan)}
.tree-row.root-service{font-weight:600}
.tree-indent{display:inline-block;flex-shrink:0}
.tree-caret{display:inline-flex;align-items:center;justify-content:center;width:16px;min-width:16px;max-width:16px;height:16px;flex-shrink:0;cursor:pointer;font-size:10px;color:var(--muted);transition:transform .12s}
.tree-caret.collapsed{transform:rotate(-90deg)}
.tree-caret.empty{visibility:hidden;pointer-events:none}
.tree-check{width:14px;height:14px;margin:0 6px 0 2px;flex-shrink:0;accent-color:var(--cyan);cursor:pointer}
.tree-icon{display:inline-flex;align-items:center;justify-content:center;width:16px;height:16px;margin-right:6px;flex-shrink:0}
.tree-name{flex:1;overflow:hidden;text-overflow:ellipsis;margin-right:8px}
.tree-badge{padding:1px 6px;border-radius:4px;font-size:10px;font-weight:700;letter-spacing:.03em;flex-shrink:0}
.badge-protected{background:#16323c;color:var(--cyan);border:1px solid #285d68}
.badge-orig{background:#1b2530;color:var(--muted);border:1px solid var(--line-soft)}
.badge-dis{background:#382214;color:#f0a068;border:1px solid #754428}
.explorer-right{display:flex;flex-direction:column;height:100%;min-height:0;overflow:hidden}
.sub-tabs{display:flex;gap:4px;border-bottom:1px solid var(--line-soft);margin-bottom:var(--space-2);padding-bottom:2px}
.sub-tab{padding:6px 14px;border:1px solid transparent;border-radius:var(--radius-sm);background:transparent;color:var(--muted);font-size:12px;font-weight:700;cursor:pointer}
.sub-tab:hover{background:#1d2935;color:var(--ink)}
.sub-tab.active{background:#1d2935;border-color:var(--line-soft);color:var(--cyan)}
.sub-panel{display:flex;flex-direction:column;gap:var(--space-3);flex:1;min-height:0;overflow-y:auto;overflow-x:hidden}
.sub-panel[hidden]{display:none}
.props-header-card{padding:8px 10px;margin-bottom:6px}
.props-table-wrap{display:flex;flex-direction:column;border:1px solid #182534;border-radius:var(--radius-sm);background:#0d151f;overflow-x:hidden}
.prop-category{background:#15212d;border-top:1px solid #1c2b3a;border-bottom:1px solid #1c2b3a;color:var(--cyan);font-size:10.5px;font-weight:700;letter-spacing:.04em;padding:3px 8px;display:flex;align-items:center;gap:6px;user-select:none}
.prop-category:first-child{border-top:none}
.prop-row{display:grid;grid-template-columns:145px 1fr;align-items:stretch;border-bottom:1px solid #14202c;font-size:11.5px;min-height:24px}
.prop-row:last-child{border-bottom:none}
.prop-name-cell{padding:3px 8px;color:#9eb0c4;background:#101a25;border-right:1px solid #162432;display:flex;align-items:center;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-weight:500;user-select:none}
.prop-name-cell:hover{color:var(--ink);background:#13202e}
.prop-val-cell{padding:2px 6px;background:#0c141d;display:flex;align-items:center;min-width:0}
.prop-val-cell input[type="text"],.prop-val-cell input[type="number"]{height:22px;font-size:11.5px;padding:1px 6px;width:100%;background:#121e2b;border:1px solid #1d2d3e;border-radius:3px;color:var(--ink)}
.prop-val-cell input[type="text"]:focus,.prop-val-cell input[type="number"]:focus{border-color:var(--cyan);background:#152433;outline:none}
.prop-color-row{display:flex;align-items:center;gap:6px;width:100%}
.prop-color-swatch{width:18px;height:18px;border-radius:3px;border:1px solid #2f455a;cursor:pointer;flex-shrink:0;padding:0;overflow:hidden;-webkit-appearance:none;background:transparent}
.prop-color-swatch::-webkit-color-swatch-wrapper{padding:0}
.prop-color-swatch::-webkit-color-swatch{border:none;border-radius:2px}
.prop-checkbox{width:15px;height:15px;accent-color:var(--cyan);cursor:pointer;margin:0}
.prop-attr-key-input{height:20px;font-size:11px;padding:1px 4px;width:95px;background:#121e2b;border:1px solid #1d2d3e;border-radius:3px;color:var(--ink)}
.prop-del-btn{width:18px;height:18px;display:flex;align-items:center;justify-content:center;border-radius:3px;background:transparent;border:1px solid transparent;color:var(--muted);cursor:pointer;padding:0;font-size:12px}
.prop-del-btn:hover{color:#f87171;background:#2b181b;border-color:#5c2429}
.prop-enum-select{height:22px;font-size:11.5px;padding:1px 4px;width:100%;background:#121e2b;border:1px solid #1d2d3e;border-radius:3px;color:var(--ink)}
.prop-enum-select:focus{border-color:var(--cyan);background:#152433;outline:none}
.filter-dropdown-wrap{position:relative;display:inline-block;flex-shrink:0}
.filter-toggle-btn{display:flex;align-items:center;gap:6px;height:28px;padding:0 9px;font-size:11.5px;border-radius:var(--radius-sm);background:#131f2c;border:1px solid #1f3347;color:var(--ink);cursor:pointer;white-space:nowrap}
.filter-toggle-btn:hover{border-color:var(--cyan);background:#172738}
.filter-badge{padding:1px 5px;border-radius:8px;font-size:10px;font-weight:700;background:#173440;color:var(--cyan);border:1px solid #235868}
.filter-dropdown-menu{position:absolute;top:calc(100% + 4px);right:0;z-index:500;min-width:180px;background:#111a24;border:1px solid #1f3246;border-radius:6px;box-shadow:0 12px 32px #000c;padding:6px;font-size:11.5px;user-select:none}
.filter-menu-header{display:flex;align-items:center;justify-content:space-between;padding:3px 6px 6px 6px;border-bottom:1px solid #1c2c3d;font-size:11px;font-weight:700;color:var(--muted)}
.filter-menu-actions{display:flex;align-items:center;gap:4px}
.filter-link-btn{background:transparent;border:none;color:var(--cyan);font-size:10.5px;cursor:pointer;padding:0;font-weight:600}
.filter-link-btn:hover{text-decoration:underline}
.filter-checkbox-list{max-height:220px;overflow-y:auto;padding-top:4px;display:flex;flex-direction:column;gap:1px}
.filter-checkbox-list::-webkit-scrollbar{width:4px}
.filter-checkbox-list::-webkit-scrollbar-thumb{background:#233446;border-radius:2px}
.filter-check-item{display:flex;align-items:center;gap:6px;padding:3px 6px;border-radius:3px;cursor:pointer;color:var(--ink);font-size:11.5px}
.filter-check-item:hover{background:#182635}
.filter-check-item input{width:13px;height:13px;accent-color:var(--cyan);margin:0;cursor:pointer}
.context-menu{position:fixed;z-index:500;min-width:145px;background:#131e2b;border:1px solid #22374d;border-radius:6px;box-shadow:0 12px 32px #000c;padding:4px;font-size:11.5px;user-select:none}
.context-menu-item{padding:5px 9px;border-radius:4px;display:flex;align-items:center;justify-content:space-between;gap:8px;cursor:pointer;color:var(--ink)}
.context-menu-item:hover{background:#1d334a;color:var(--cyan)}
.context-menu-item.danger:hover{background:#3b171c;color:#f87171}
.context-menu-divider{height:1px;background:#1f3042;margin:3px 0}
.icon-button.danger-hover:hover{color:#f87171;border-color:#5c2429;background:#2b181b}
.protect-card{padding:10px 12px}
.terminal-console{overflow-y:auto;background:#080e15;border:1px solid #162432;border-radius:4px;padding:8px 10px;font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,"Liberation Mono",monospace;font-size:11px;line-height:1.5;color:#94a3b8}
.terminal-console::-webkit-scrollbar{width:5px}
.terminal-console::-webkit-scrollbar-thumb{background:#1c2d3e;border-radius:3px}
.console-line{margin-bottom:3px;overflow-wrap:anywhere;white-space:pre-wrap}
.console-line.protect{color:#4ade80}
.console-line.restore{color:#38bdf8}
.console-line.state{color:#fbbf24}
.console-line.error{color:#f87171}
.console-line.info{color:#94a3b8}
.console-tag{display:inline-block;padding:0 4px;border-radius:2px;font-weight:700;font-size:9.5px;margin-right:4px;vertical-align:baseline}
.console-tag.protect{background:#143425;color:#4ade80;border:1px solid #236544}
.console-tag.restore{background:#122c3e;color:#38bdf8;border:1px solid #1c5270}
.console-tag.state{background:#352912;color:#fbbf24;border:1px solid #634e23}
.console-tag.error{background:#36161b;color:#f87171;border:1px solid #6c2a32}
.console-tag.info{background:#152230;color:#94a3b8;border:1px solid #20364e}
[hidden]{display:none!important}
.tree-counts{display:flex;gap:var(--space-1);margin-bottom:0}
.tree-count-pill{padding:2px 7px;border-radius:12px;font-size:11px;font-weight:700;border:1px solid var(--line-soft);background:var(--well)}
.tree-count-pill.cyan{border-color:var(--cyan);color:var(--cyan)}
.tree-count-pill.green{border-color:var(--green);color:var(--green)}
.tree-count-pill.amber{border-color:#f0a068;color:#f0a068}
.empty{display:grid;place-items:center;min-height:100px;color:var(--muted);text-align:center;padding:var(--space-6)}
.modal-backdrop{position:fixed;inset:0;z-index:200;display:none;place-items:center;padding:var(--space-6);background:#050a10c7}.modal-backdrop.open{display:grid}.modal{width:min(780px,100%);max-height:min(680px,90vh);display:grid;grid-template-rows:auto minmax(0,1fr);gap:var(--space-3);border:1px solid var(--line);border-radius:var(--radius-lg);background:var(--shell);box-shadow:0 24px 70px #000b;padding:var(--space-4)}.modal-head{display:flex;align-items:center;justify-content:space-between;gap:var(--space-3)}.modal-head h2{font-size:20px}.modal-body{min-height:0;overflow:auto}.modal-actions{display:flex;gap:var(--space-2)}.meta-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:var(--space-2);margin-bottom:var(--space-3)}.meta-cell{min-width:0;padding:var(--space-2);border:1px solid var(--line-soft);border-radius:var(--radius-sm);background:var(--well);overflow-wrap:anywhere}.add-form{display:grid;gap:var(--space-3);min-width:0}.toast{position:fixed;right:var(--space-4);bottom:var(--space-4);z-index:300;max-width:min(420px,80vw);display:none;align-items:center;gap:var(--space-2);padding:var(--space-3) var(--space-4);border:1px solid var(--line);border-radius:var(--radius-sm);background:#192733;color:var(--ink);box-shadow:var(--shadow)}.toast.open{display:flex}.toast.success{border-color:#438d65;background:#173426;color:#dff9e8}.toast.error{border-color:#9a5961;background:#452a30;color:#ffe4e7}.toast.warning{border-color:#8f753d;background:#3c321e;color:#fff0c7}.toast.info{border-color:#477f8a;background:#173139;color:#dff9fb}.toast-icon{display:grid;place-items:center;flex:0 0 20px;width:20px;height:20px;font-weight:850}.toast-message{min-width:0;overflow-wrap:anywhere}.sr-only{position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0}
@media(max-width:980px){.brand small,.title-context .address{display:none}.health-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.history-shell,.exec-shell,.config-shell,.protection-shell{grid-template-columns:1fr}.history-shell,.exec-shell,.config-shell,.protection-shell{overflow:auto}.exec-editor,.exec-result{min-height:430px}.instance-actions .optional{display:none}}
@media(max-width:680px){.brand{min-width:auto}.brand div{display:none}.state-text{display:none}.metrics{grid-template-columns:1fr}.summary-card{grid-template-columns:1fr}.exec-toolbar,.form-grid{grid-template-columns:1fr}.content{padding:var(--space-2)}.tab{min-width:82px}.meta-grid,.health-grid{grid-template-columns:1fr}.instance-actions button{padding-inline:9px}.summary-activity{grid-template-columns:1fr}}
@media(prefers-reduced-motion:reduce){*,*:before,*:after{scroll-behavior:auto!important;transition-duration:.01ms!important;animation-duration:.01ms!important;animation-iteration-count:1!important}}
</style>
</head>
<body>
<div id="app" class="app">
  <header id="appHeader" class="titlebar glass">
    <div class="brand"><img class="brand-logo" src="data:image/png;base64,` + brandIcon + `" alt="" aria-hidden="true"><div><strong>RiftSync</strong><span class="brand-version">v` + serverapp.Version + `</span><small>ROBLOX STUDIO LIVE SYNC</small></div></div>
    <div class="title-context" aria-live="polite"><span id="titleLamp" class="lamp" aria-hidden="true"></span><span id="titleName" class="title-name">No project</span><span id="titleState" class="state-text">Stopped</span><span id="titleAddress" class="address">—</span></div>
    <div class="instance-actions" aria-label="Selected project actions"><button id="logsBtn" class="optional">Logs</button><button id="startBtn" class="primary">Start</button><button id="stopBtn">Stop</button></div>
    <div class="window-actions"><button id="minimizeBtn" class="icon-button" aria-label="Minimize window">—</button><button id="closeBtn" class="icon-button danger" aria-label="Close RiftSync">×</button></div>
  </header>
  <aside class="sidebar-wrap">
    <nav class="sidebar glass" aria-label="Projects">
      <div class="sidebar-head"><button id="pinBtn" class="icon-button" title="Pin sidebar" aria-label="Pin project sidebar" aria-expanded="false">☰</button><strong>Projects</strong></div>
      <div id="projectList" class="project-list"></div>
      <div class="sidebar-foot">
        <button id="startAllBtn" class="side-action"><span class="side-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path fill="currentColor" d="M8 5.5v13l10-6.5z"/></svg></span><span class="action-label">Start All</span></button>
        <button id="addProjectBtn" class="side-action primary"><span class="side-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" d="M12 5v14M5 12h14"/></svg></span><span class="action-label">Add Project</span></button>
      </div>
    </nav>
  </aside>
  <main class="workspace" aria-label="Selected project workspace">
    <div class="toolbar">
      <div class="tabs" role="tablist" aria-label="Project workspace">
        <button id="overviewTab" class="tab active" role="tab" aria-controls="overviewPanel" aria-selected="true" tabindex="0" data-panel="overviewPanel">Overview</button>
        <button id="explorerTab" class="tab" role="tab" aria-controls="explorerPanel" aria-selected="false" tabindex="-1" data-panel="explorerPanel">Explorer</button>
        <button id="historyTab" class="tab" role="tab" aria-controls="historyPanel" aria-selected="false" tabindex="-1" data-panel="historyPanel">History</button>
        <button id="execTab" class="tab" role="tab" aria-controls="execPanel" aria-selected="false" tabindex="-1" data-panel="execPanel">Exec</button>
        <button id="configTab" class="tab" role="tab" aria-controls="configPanel" aria-selected="false" tabindex="-1" data-panel="configPanel">Config</button>
      </div>
    </div>
    <div class="content">
      <section id="overviewPanel" class="panel-page active" role="tabpanel" aria-labelledby="overviewTab" tabindex="0">
        <div class="overview">
          <article id="workspacePanel" class="panel summary-card"><div><div class="label">Active project</div><h1 id="projectNameText" class="project-name">No project</h1><div id="projectPathText" class="path">—</div><div class="button-row"><button id="openBtn">Open Folder</button><button id="changePathBtn">Edit Settings</button></div></div><div class="summary-state"><span id="overviewLamp" class="lamp stopped" aria-hidden="true"></span><div><strong id="overviewStatusText">Stopped</strong><div id="uptimeText" class="hint">Not running</div></div></div><div class="summary-activity"><div class="label">Last activity</div><div><div id="activityText" class="truncate" aria-live="polite">No Studio activity yet.</div><div id="activityHint" class="hint"></div><div id="activityProgress" class="activity-progress" role="progressbar" aria-label="Sync activity progress" aria-valuemin="0" aria-valuemax="100" aria-busy="false" hidden><span id="activityProgressFill" class="activity-progress-fill"></span></div><div style="display:flex;align-items:center;gap:8px"><div id="errorText" class="activity-notice success" role="status" aria-live="polite" aria-atomic="true" style="flex:1">✓ No issues</div><button id="errorDetailsBtn" type="button" hidden>Details</button></div></div></div></article>
          <div id="metricStrip" class="metrics" aria-label="Project metrics">
            <div class="metric"><div class="label">Revision</div><div id="revisionText" class="value">0</div><div id="modeText" class="hint">Live sync</div></div>
            <div class="metric"><div class="label">Indexed</div><div id="indexedText" class="value">0</div><div id="indexedHint" class="hint">0 scripts · 0 UI</div></div>
            <div class="metric"><div class="label">Git</div><div id="gitText" class="value">Pending</div><div class="hint">Project history</div></div>
          </div>
          <article id="healthPanel" class="panel disclosure"><button id="diagnosticsToggle" class="disclosure-trigger" aria-expanded="false" aria-controls="diagnosticsBody"><span><span class="label">Diagnostics</span><span class="hint">Connection and watcher details</span></span><span class="sr-only">Toggle diagnostics</span></button><div id="diagnosticsBody" class="disclosure-body" hidden><div class="health-grid">
            <div class="health-cell"><div class="label">Studio polls</div><div id="pollsText" class="value">0</div></div>
            <div class="health-cell"><div class="label">App requests</div><div id="requestsText" class="value">0</div></div>
            <div class="health-cell"><div class="label">Watcher</div><div id="healthModeText" class="value">Live</div></div>
            <div class="health-cell"><div class="label">Last scan</div><div id="lastScanText" class="value">Ready</div></div>
          </div></div></article>
        </div>
      </section>
      <section id="explorerPanel" class="panel-page" role="tabpanel" aria-labelledby="explorerTab" tabindex="0" hidden>
        <div class="protection-shell">
          <article class="panel explorer-panel">
            <div class="explorer-header">
              <div><h2 style="margin:0;font-size:16px">Roblox Explorer</h2><div class="hint" style="margin-top:2px">Studio hierarchy & properties</div></div>
              <div class="tree-counts">
                <span id="protTotalBadge" class="tree-count-pill">0 Scripts</span>
                <span id="protProtectedBadge" class="tree-count-pill cyan">0 Protected</span>
                <span id="protOrigBadge" class="tree-count-pill green">0 Original</span>
                <span id="protDisabledBadge" class="tree-count-pill amber">0 Disabled</span>
              </div>
            </div>
            <div class="explorer-search-bar">
              <input id="explorerSearchInput" type="text" placeholder="Search explorer (e.g. Combat, Module, ...)" spellcheck="false">
              <div class="filter-dropdown-wrap">
                <button id="serviceFilterDropdownBtn" class="filter-toggle-btn" type="button" title="Filter Visible Services">
                  <span>Services</span> <span id="serviceFilterBadge" class="filter-badge">All</span> <span style="font-size:9px">▼</span>
                </button>
                <div id="serviceFilterDropdownMenu" class="filter-dropdown-menu" hidden>
                  <div class="filter-menu-header">
                    <span>Visible Services</span>
                    <div class="filter-menu-actions">
                      <button id="filterSelectAllBtn" class="filter-link-btn" type="button">All</button>
                      <span style="color:var(--muted);font-size:10px">|</span>
                      <button id="filterDeselectAllBtn" class="filter-link-btn" type="button">None</button>
                    </div>
                  </div>
                  <div id="serviceFilterList" class="filter-checkbox-list"></div>
                </div>
              </div>
              <button id="explorerRefreshBtn" class="icon-button" title="Refresh tree">⟳</button>
            </div>
            <div id="explorerTreeWrap" class="explorer-tree-wrap">
              <div class="empty">Loading explorer tree...</div>
            </div>
          </article>
          <aside class="explorer-right">
            <div class="sub-tabs" role="tablist">
              <button id="subTabProps" class="sub-tab active" type="button">Properties</button>
              <button id="subTabSecurity" class="sub-tab" type="button">Security</button>
            </div>
            <!-- Sub-Panel 1: Properties Inspector -->
            <div id="propsSubPanel" class="sub-panel">
              <article class="panel protect-card props-header-card">
                <div style="display:flex;align-items:center;justify-content:space-between;gap:8px">
                  <div style="display:flex;align-items:center;gap:6px;min-width:0;flex:1">
                    <span id="selectedNodeIcon" class="tree-icon" style="margin:0;width:16px;height:16px"></span>
                    <span id="selectedNodeClassPill" class="tree-count-pill cyan" style="font-size:10px;padding:1px 5px">No Selection</span>
                    <strong id="selectedNodeInfo" style="font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">Select an item</strong>
                    <div id="selectedNodeBadges" style="display:flex;gap:4px"></div>
                  </div>
                  <div style="display:flex;align-items:center;gap:4px;flex-shrink:0">
                    <button id="toggleSingleEnabledBtn" class="optional" style="padding:2px 8px;font-size:11px;height:24px" hidden>Disable Script</button>
                    <button id="renameNodeBtn" class="optional icon-button" style="width:24px;height:24px;font-size:11px" title="Rename (F2)" disabled>✎</button>
                  </div>
                </div>
                <div id="selectedNodePath" class="path truncate" style="margin-top:4px;font-size:10.5px;color:var(--muted)">—</div>
              </article>

              <article class="panel protect-card" style="flex:1;min-height:0;display:flex;flex-direction:column;padding:8px 10px">
                <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:6px">
                  <h2 style="margin:0;font-size:13px;font-weight:700">Properties Inspector</h2>
                  <button id="savePropsBtn" class="primary" style="padding:2px 10px;font-size:11.5px" disabled>Save Properties</button>
                </div>
                <div id="propertiesForm" class="props-table-wrap" style="flex:1;overflow-y:auto;overflow-x:hidden">
                  <div class="hint" style="text-align:center;padding:24px 0">Select an item in Explorer to view properties.</div>
                </div>
              </article>
            </div>

            <!-- Sub-Panel 2: Security & Protection -->
            <div id="securitySubPanel" class="sub-panel" hidden style="padding-bottom:8px">
              <!-- Card 1: VM Protection Control -->
              <article class="panel protect-card" style="flex-shrink:0;padding:10px 12px">
                <div style="display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:6px">
                  <div style="display:flex;align-items:center;gap:6px;min-width:0">
                    <span class="tree-icon" style="margin:0;width:16px;height:16px"><svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#16323c"/><path d="M8 2.5L3.5 4.5v4c0 3 2.5 5 4.5 5.5 2-.5 4.5-2.5 4.5-5.5v-4L8 2.5z" fill="#38bdf8"/></svg></span>
                    <h2 style="font-size:13px;font-weight:700;margin:0">VM Protection</h2>
                  </div>
                  <div id="smartTargetBadge" class="tree-count-pill" style="font-size:10px;padding:2px 7px;max-width:170px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">No Selection</div>
                </div>

                <div id="smartTargetDesc" style="font-size:11px;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap;margin-bottom:6px">Select a script or check items in Explorer</div>

                <div class="field" style="margin-top:4px">
                  <label class="label" for="protWatermarkInput" style="font-size:10px">Watermark / Header (Optional)</label>
                  <input id="protWatermarkInput" placeholder="-- Protected by Sorevium VM" spellcheck="false" style="height:26px;font-size:11.5px">
                </div>

                <!-- Unified Smart Action Row: Protect & Restore -->
                <div class="button-row" style="margin-top:8px;gap:6px">
                  <button id="smartProtectBtn" class="primary" style="flex:1;height:28px;font-size:11.5px" disabled>Protect VM</button>
                  <button id="smartRestoreBtn" style="flex:1;height:28px;font-size:11.5px" disabled>Restore Original</button>
                </div>
              </article>

              <!-- Card 2: Modern Activity Console -->
              <article class="panel protect-card" style="flex:1;min-height:160px;display:flex;flex-direction:column;padding:8px 10px;overflow:hidden">
                <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:6px">
                  <div style="display:flex;align-items:center;gap:6px">
                    <span class="lamp active" style="width:7px;height:7px;background:#4ade80;box-shadow:0 0 6px #4ade80;border-radius:50%"></span>
                    <h2 style="font-size:12.5px;font-weight:700;margin:0">Activity Console</h2>
                  </div>
                  <button id="clearConsoleBtn" class="filter-link-btn" type="button" style="font-size:11px">Reset</button>
                </div>
                <div id="securityConsole" class="terminal-console" style="flex:1;min-height:0">
                  <div class="console-line"><span style="color:#64748b">[INIT]</span> RiftSync Protection & Explorer Console ready.</div>
                </div>
              </article>
            </div>
          </aside>
        </div>
      </section>
      <section id="historyPanel" class="panel-page" role="tabpanel" aria-labelledby="historyTab" tabindex="0" hidden>
        <div class="history-shell"><aside class="panel history-list"><div class="recent-head"><div><h2>Revisions</h2><div class="hint">Selected project only</div></div><button id="historyRefreshBtn">Refresh</button></div><div id="historyTimeline"><div class="empty">No revisions yet.</div></div></aside><article class="panel history-detail"><div id="revisionSummary" class="hint">Choose a revision.</div><div id="historyDetail"></div></article></div>
      </section>
      <section id="execPanel" class="panel-page" role="tabpanel" aria-labelledby="execTab" tabindex="0" hidden>
        <div class="exec-shell">
          <article class="panel exec-editor"><div><h2>Remote Exec</h2><div class="hint">Runs only through the selected project.</div></div><div><label class="label" for="execTokenDisplay">Project token</label><div class="token-row"><input id="execTokenDisplay" readonly><button id="execTokenCopyBtn">Copy</button></div><div id="execTokenHint" class="hint"></div></div><label class="sr-only" for="execSourceInput">Luau source</label><textarea id="execSourceInput" spellcheck="false" placeholder="print(workspace.Name)"></textarea><div class="exec-toolbar"><div><label class="label" for="execTimeoutInput">Timeout</label><input id="execTimeoutInput" type="number" min="1" max="120" value="10"></div><div id="execStatusText" class="hint" role="status" aria-live="polite">Start project and enable Exec in Studio.</div><button id="execLatestBtn">Latest</button><button id="execRunBtn" class="primary">Run</button></div></article>
          <article class="panel exec-result"><div><h2>Result</h2><div id="execResultHint" class="hint">Ready.</div></div><pre id="execOutputText" class="output">Ready.</pre><div class="recent-head"><div><div class="label">Recent commands</div><div id="execHistoryHint" class="hint">Per-project history</div></div><button id="execHistoryRefreshBtn">Refresh</button></div><div id="execHistoryList" class="exec-history"><div class="empty">No commands yet.</div></div></article>
        </div>
      </section>
      <section id="configPanel" class="panel-page" role="tabpanel" aria-labelledby="configTab" tabindex="0" hidden>
        <div class="config-shell"><article id="configPathPanel" class="panel form-panel"><h2>Project</h2><div class="hint">Identity and local folder for this instance.</div><div class="field"><label class="label" for="instanceNameInput">Display name</label><div class="input-button"><input id="instanceNameInput"><button id="renameBtn">Rename</button></div></div><div class="field"><label class="label" for="syncRootInput">Sync root</label><div class="input-button"><input id="syncRootInput"><button id="syncRootPickerBtn">Choose</button></div></div><div class="button-row"><button id="applyConfigBtn" class="primary">Save Settings</button><button id="configOpenBtn">Open Folder</button></div><div id="configApplyStatus" class="hint" role="status" aria-live="polite">Saved</div></article><aside class="config-side"><article id="serverSettingsPanel" class="panel"><h2>Connection</h2><div class="hint">Use a unique port for each project, then copy its token into Studio.</div><div class="form-grid"><div class="field"><label class="label" for="hostInput">Host</label><input id="hostInput"></div><div class="field"><label class="label" for="portInput">Port</label><input id="portInput" inputmode="numeric"></div></div><div id="connectionSummary" class="path" style="margin-top:12px">—</div></article><article class="panel disclosure"><button id="advancedToggle" class="disclosure-trigger" aria-expanded="false" aria-controls="advancedBody"><span><span class="label">Advanced</span><span class="hint">Config path and debug mode</span></span><span class="sr-only">Toggle advanced settings</span></button><div id="advancedBody" class="disclosure-body" hidden><div class="field"><label class="label" for="configInput">Config path</label><input id="configInput" readonly></div><div class="field toggle-row"><span><strong>Debug mode</strong><span class="hint">Extra runtime diagnostics</span></span><input id="debugInput" type="checkbox" aria-label="Debug mode"></div></div></article><article class="panel danger-zone"><h2>Remove project</h2><div class="hint">Only remove this project from RiftSync. Files and configuration remain untouched.</div><div class="button-row"><button id="removeProjectBtn" class="danger">Remove Project</button></div></article></aside></div>
      </section>
    </div>
  </main>
</div>
<div id="viewModal" class="modal-backdrop" aria-hidden="true"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="viewTitle" tabindex="-1"><header class="modal-head"><div><h2 id="viewTitle" style="margin:0">Command details</h2><div id="viewSubtitle" class="hint"></div></div><div class="modal-actions"><button id="viewCopyBtn">Copy Source</button><button id="viewCloseBtn">Close</button></div></header><div class="modal-body"><div id="viewMeta" class="meta-grid"></div><div class="label">Source</div><pre id="viewSource" class="code-view" style="min-height:180px"></pre><div class="label" style="margin-top:12px">Last result</div><pre id="viewResult" class="code-view" style="min-height:90px"></pre></div></section></div>
<div id="addModal" class="modal-backdrop" aria-hidden="true"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="addTitle" tabindex="-1" style="width:min(560px,100%)"><header class="modal-head"><div><h2 id="addTitle" style="margin:0">Add project</h2><div class="hint">Import an existing config or create a local-only config.</div></div><button id="addCloseBtn">Close</button></header><div class="modal-body add-form"><div><label class="label" for="addNameInput">Display name (optional)</label><input id="addNameInput"></div><div><label class="label" for="addRootInput">Sync root</label><div class="input-button"><input id="addRootInput"><button id="addPickBtn">Choose</button></div></div><div><label class="label" for="addConfigInput">Existing config path (optional)</label><input id="addConfigInput" placeholder="Leave empty to use .rblxsync\\sync_config.json"></div><button id="addCreateBtn" class="primary">Add Project</button><div id="addStatus" class="hint" role="status" aria-live="polite"></div></div></section></div>
<div id="removeModal" class="modal-backdrop" aria-hidden="true"><section class="modal" role="alertdialog" aria-modal="true" aria-labelledby="removeTitle" aria-describedby="removeDescription" tabindex="-1" style="width:min(520px,100%)"><header class="modal-head"><div><h2 id="removeTitle" style="margin:0">Remove project?</h2><div id="removeProjectName" class="hint"></div></div></header><div class="modal-body add-form"><p id="removeDescription" class="hint" style="margin:0">Choose whether RiftSync should delete the project folder or only remove this project from the app.</p><div class="modal-actions" style="justify-content:flex-end;flex-wrap:wrap"><button id="removeCancelBtn">Cancel</button><button id="removeKeepBtn">Remove, keep files</button><button id="removeDeleteBtn" class="danger">Delete project files</button></div><div id="removeStatus" class="hint" role="status" aria-live="polite"></div></div></section></div>
<div id="logsModal" class="modal-backdrop" aria-hidden="true"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="logsTitle" tabindex="-1"><header class="modal-head"><div><h2 id="logsTitle" style="margin:0">Project logs</h2><div class="hint">Selected project only</div></div><div class="modal-actions"><button id="refreshLogsBtn">Refresh</button><button id="closeLogsBtn">Close</button></div></header><pre id="logsText" class="code-view"></pre></section></div>
<div id="errorModal" class="modal-backdrop" aria-hidden="true"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="errorTitle" tabindex="-1"><header class="modal-head"><div><h2 id="errorTitle" style="margin:0">Sync conflict details</h2><div class="hint">Full preflight/apply/rollback report</div></div><div class="modal-actions"><button id="copyErrorBtn">Copy report</button><button id="closeErrorBtn">Close</button></div></header><pre id="errorDetailText" class="code-view"></pre></section></div>
<div id="renameModal" class="modal-backdrop" aria-hidden="true"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="renameTitle" tabindex="-1" style="width:min(440px,100%)"><header class="modal-head"><div><h2 id="renameTitle" style="margin:0;font-size:16px">Rename Instance</h2><div id="renameSub" class="hint">Enter new name</div></div></header><div class="modal-body add-form"><div class="field"><label class="label" for="renameInput">New Name</label><input id="renameInput" type="text" spellcheck="false"></div><div class="modal-actions" style="justify-content:flex-end"><button id="cancelRenameBtn" type="button">Cancel</button><button id="confirmRenameBtn" class="primary" type="button">Rename</button></div></div></section></div>
<div id="deleteNodeModal" class="modal-backdrop" aria-hidden="true"><section class="modal" role="alertdialog" aria-modal="true" aria-labelledby="deleteNodeTitle" tabindex="-1" style="width:min(460px,100%)"><header class="modal-head"><div><h2 id="deleteNodeTitle" style="margin:0;font-size:16px;color:#f87171">Delete Instance?</h2><div id="deleteNodeSub" class="hint">This action cannot be undone</div></div></header><div class="modal-body add-form"><p id="deleteNodeMsg" style="font-size:12px;margin:0">Are you sure you want to delete this item from disk and Studio?</p><div class="modal-actions" style="justify-content:flex-end"><button id="cancelDeleteNodeBtn" type="button">Cancel</button><button id="confirmDeleteNodeBtn" class="danger" type="button">Delete</button></div></div></section></div>
<div id="explorerContextMenu" class="context-menu" style="display:none"></div>
<div id="toast" class="toast info" role="status" aria-live="polite" aria-atomic="true"><span id="toastIcon" class="toast-icon" aria-hidden="true">i</span><span id="toastMessage" class="toast-message"></span></div>
<script>
const $=id=>document.getElementById(id);
let instances=[],selectedId="",latest=null,execHistory=[],selectedEntry=null,busy=false,modalReturnFocus=null,configDirty=false,configApplying=false,pendingRemoveProjectId="";
const scopedPaths=new Set(["/app/status","/app/start","/app/stop","/app/restart","/app/config/apply","/app/open-folder","/app/pick-folder","/app/history","/app/exec/history","/app/exec/run","/app/exec/rerun","/app/logs","/app/protection/tree","/app/protection/protect","/app/protection/restore","/app/explorer/tree","/app/explorer/protect","/app/explorer/restore","/app/explorer/set-enabled","/app/explorer/set-properties","/app/explorer/reveal","/app/explorer/rename","/app/explorer/delete"]);
function withInstance(path){const scoped=[...scopedPaths].some(base=>path===base||path.startsWith(base+"?"));if(!scoped||!selectedId)return path;return path+(path.includes("?")?"&":"?")+"instance_id="+encodeURIComponent(selectedId)}
async function api(path,options){const response=await fetch(withInstance(path),options);const text=await response.text();let body={};try{body=JSON.parse(text)}catch(_){throw new Error(text||"Invalid response")}if(!response.ok||body.status==="error")throw new Error(body.error||body.message||("HTTP "+response.status));return body}
function escapeHtml(value){return String(value??"").replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#039;"}[c]))}
function hashColor(value){let h=0;for(const c of String(value))h=(h*31+c.charCodeAt(0))>>>0;const colors=["#49607d","#58607f","#3e716c","#71556c","#75633f","#4d6d56"];return colors[h%colors.length]}
function initials(name){const bits=String(name||"?").trim().split(/\s+/);return(bits.length>1?bits[0][0]+bits[bits.length-1][0]:bits[0].slice(0,2)).toUpperCase()}
function toast(message,kind="info"){const variants={success:{icon:"✓",role:"status",live:"polite"},error:{icon:"!",role:"alert",live:"assertive"},warning:{icon:"!",role:"status",live:"polite"},info:{icon:"i",role:"status",live:"polite"}},variant=variants[kind]||variants.info,node=$("toast");node.className="toast open "+(variants[kind]?kind:"info");node.setAttribute("role",variant.role);node.setAttribute("aria-live",variant.live);$("toastIcon").textContent=variant.icon;$("toastMessage").textContent=String(message||"");clearTimeout(node._timer);node._timer=setTimeout(()=>node.classList.remove("open"),3500)}
function current(){return instances.find(item=>item.id===selectedId)||null}
function lampClass(app){return app&&app.starting?"starting":app&&app.syncing?"syncing":app&&app.running?"running":app&&app.last_error?"error":"stopped"}
function activityDetails(app){const details=app&&app.activity_details;if(!details||typeof details!=="object"||!Object.keys(details).length)return"";try{return JSON.stringify(details,null,2)}catch(_){return""}}
function renderIssue(app){const node=$("errorText"),message=String(app.last_error||"").trim(),raw=app.activity_details,details=activityDetails(app),full=message+(details?"\n\n"+details:""),conflicts=raw&&Array.isArray(raw.conflicts)&&raw.conflicts.length>0,rollbackFailed=raw&&raw.rollback_status==="failed",hasIssue=!!message||app.activity_error===true||conflicts||rollbackFailed||raw&&raw.phase==="rollback",state=hasIssue?"error":"success",text=hasIssue?"Error: "+(message||"Sync failed; open Details"):"✓ No issues",button=$("errorDetailsBtn");if(node.dataset.state!==state||node.textContent!==text){node.dataset.state=state;node.className="activity-notice "+state;node.setAttribute("role",hasIssue?"alert":"status");node.setAttribute("aria-live",hasIssue?"assertive":"polite");node.textContent=text}button.hidden=!full;button.disabled=!full;button.dataset.report=full}
function phaseLabel(value){return({queued:"Queued",loading_config:"Loading config",reserving_port:"Reserving port",preparing_project:"Preparing project",enumerating:"Finding files",scanning:"Scanning files",indexing_state:"Indexing state",loading_history:"Loading history",loading_project_state:"Loading project state",starting_git:"Starting Git",starting_watcher:"Starting watcher",starting_server:"Starting server",preflight:"Preflight",apply:"Apply",rollback:"Rollback",complete:"Complete",error:"Failed"})[value]||value||""}
function renderActivity(app){const progress=Math.max(0,Math.min(100,Number(app.activity_progress)||0)),current=Math.max(0,Number(app.activity_current)||0),total=Math.max(0,Number(app.activity_total)||0),indeterminate=!!app.activity_indeterminate,track=$("activityProgress"),fill=$("activityProgressFill");$("activityText").textContent=app.activity_text||"No Studio activity yet.";const bits=[phaseLabel(app.activity_phase||app.activity_operation),total?current+"/"+total:"",progress?progress+"%":"",app.activity_revision?"rev "+app.activity_revision:""].filter(Boolean);$("activityHint").textContent=bits.join(" · ");track.hidden=!(indeterminate||progress>0);track.classList.toggle("indeterminate",indeterminate);track.setAttribute("aria-busy",String(indeterminate));if(indeterminate){track.removeAttribute("aria-valuenow");fill.style.width=progress?progress+"%":""}else{track.setAttribute("aria-valuenow",String(progress));fill.style.width=progress+"%"}}
function renderSidebar(body){instances=body.instances||[];selectedId=body.selected_instance_id||instances[0]?.id||"";const pinned=!!body.sidebar_pinned;$("app").classList.toggle("sidebar-pinned",pinned);if(pinned)sidebarWrap.classList.remove("force-collapsed");$("pinBtn").textContent=pinned?"⇤":"☰";$("pinBtn").setAttribute("aria-expanded",String(pinned));$("pinBtn").setAttribute("aria-label",pinned?"Unpin project sidebar":"Pin project sidebar");$("projectList").innerHTML=instances.length?instances.map(item=>{const a=item.app||{},active=item.id===selectedId,status=a.status||"Stopped";return '<button class="project-button '+(active?"active":"")+'" data-id="'+escapeHtml(item.id)+'" aria-current="'+(active?"page":"false")+'" aria-label="'+escapeHtml(item.name+", "+status+", port "+(a.port||"not set"))+'"><span class="avatar" style="--avatar:'+hashColor(item.id)+'">'+escapeHtml(initials(item.name))+'<span class="lamp '+lampClass(a)+'" aria-hidden="true"></span></span><span class="project-copy"><strong>'+escapeHtml(item.name)+'</strong><span>'+escapeHtml(status)+' · '+escapeHtml(a.port||"—")+'</span></span></button>'}).join(""):'<div class="empty">Add your first project.</div>';$("projectList").querySelectorAll("[data-id]").forEach(node=>node.onclick=()=>selectInstance(node.dataset.id));if(body.warning)toast(body.warning,"warning")}
async function loadInstances(renderSelected=true){try{const body=await api("/app/instances");renderSidebar(body);if(renderSelected)await refreshSelected()}catch(err){toast(err.message,"error")}}
async function selectInstance(id){if(id===selectedId)return;await api("/app/instances/select",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({id})});selectedId=id;execHistory=[];configDirty=false;configApplying=false;checkedPaths.clear();selectedNode=null;enabledServices=null;await loadInstances(true);await loadHistory();await loadExecHistory();await loadProtectionTree()}
function setDisabled(noProject){["startBtn","stopBtn","logsBtn","openBtn","changePathBtn","historyRefreshBtn","execRunBtn","execLatestBtn","execHistoryRefreshBtn","execTokenCopyBtn","applyConfigBtn","renameBtn","removeProjectBtn","smartProtectBtn","smartRestoreBtn","explorerRefreshBtn","savePropsBtn"].forEach(id=>{const n=$(id);if(n)n.disabled=noProject||busy})}
function render(app){latest=app||null;const item=current();if(!app||!item){$("titleName").textContent="No project";$("titleState").textContent="Stopped";$("titleAddress").textContent="—";$("titleLamp").className="lamp stopped";$("overviewLamp").className="lamp stopped";$("projectNameText").textContent="No project";setDisabled(true);configDirty=false;configApplying=false;return}setDisabled(false);const locked=app.running||app.starting;if(locked){configDirty=false;configApplying=false}const stateClass=lampClass(app);$("titleName").textContent=item.name;$("titleState").textContent=app.status||"Stopped";$("titleAddress").textContent=app.address||"—";$("titleLamp").className="lamp "+stateClass;$("overviewLamp").className="lamp "+stateClass;$("overviewStatusText").textContent=app.status;$("uptimeText").textContent=app.uptime?"Up "+app.uptime:"Not running";$("revisionText").textContent=app.revision||0;$("modeText").textContent=app.mode||"Live sync";$("indexedText").textContent=app.indexed||0;$("indexedHint").textContent=(app.scripts||0)+" scripts · "+(app.ui||0)+" UI";$("gitText").textContent=app.git_status||"Pending";$("gitText").title=app.git_last_error||"";$("projectNameText").textContent=item.name;$("projectPathText").textContent=app.sync_root||"—";$("pollsText").textContent=app.polls||0;$("requestsText").textContent=app.request_count||0;$("healthModeText").textContent=app.mode||"Live";$("lastScanText").textContent=app.indexed?(app.indexed+" items"):"Ready";renderActivity(app);renderIssue(app);$("startBtn").disabled=busy||app.running||app.starting;$("stopBtn").disabled=busy||!app.running;$("execTokenDisplay").value=app.remote_exec_token||"";$("execTokenHint").textContent=app.remote_exec_available?"Copy token into the selected Studio profile.":"Remote Exec will be prepared when the project starts.";$("instanceNameInput").value=item.name;if(!configDirty&&!configApplying){$("syncRootInput").value=app.sync_root||"";$("configInput").value=item.config_path||app.config_path||"";$("hostInput").value=app.host||"127.0.0.1";$("portInput").value=app.port||8765;$("debugInput").checked=!!app.debug}$("connectionSummary").textContent=(app.host||"127.0.0.1")+":"+(app.port||8765);["syncRootInput","syncRootPickerBtn","hostInput","portInput","debugInput","applyConfigBtn"].forEach(id=>$(id).disabled=locked||busy||configApplying);$("configApplyStatus").textContent=locked?"Stop this project before editing settings.":configApplying?"Saving...":configDirty?"Unsaved changes":"Ready to edit."}
async function refreshSelected(){if(!selectedId){render(null);return}try{const body=await api("/app/status");render(body.app)}catch(err){toast(err.message,"error")}}
function showStartPending(){const app=latest||{};render({...app,starting:true,running:false,status:"Starting",activity_error:false,activity_text:"Starting RiftSync...",activity_operation:"startup",activity_phase:"queued",activity_progress:1,activity_current:0,activity_total:0,activity_indeterminate:true})}
async function action(path){if(!selectedId)return;busy=true;if(path==="/app/start")showStartPending();else render(latest);try{const body=await api(path,{method:"POST"});if(body.app)render(body.app);await loadInstances(false)}catch(err){toast(err.message,"error")}finally{busy=false;await refreshSelected()}}
function setPanel(id,focus=false){document.querySelectorAll(".tab").forEach(t=>{const active=t.dataset.panel===id;t.classList.toggle("active",active);t.setAttribute("aria-selected",String(active));t.tabIndex=active?0:-1;if(active&&focus)t.focus()});document.querySelectorAll(".panel-page").forEach(p=>{const active=p.id===id;p.classList.toggle("active",active);p.hidden=!active});if(id==="historyPanel")loadHistory();if(id==="execPanel")loadExecHistory();if(id==="explorerPanel"||id==="protectionPanel")loadProtectionTree()}
function setDisclosure(buttonId,bodyId,open){const button=$(buttonId),body=$(bodyId);button.setAttribute("aria-expanded",String(open));body.hidden=!open}
function openModal(id,trigger=document.activeElement){const node=$(id);modalReturnFocus=trigger;$("app").setAttribute("aria-hidden","true");$("app").inert=true;node.classList.add("open");node.setAttribute("aria-hidden","false");const target=node.querySelector("button,input,textarea,[tabindex]:not([tabindex='-1'])")||node.querySelector(".modal");target?.focus()}
function closeModal(id){const node=$(id);node.classList.remove("open");node.setAttribute("aria-hidden","true");$("app").removeAttribute("aria-hidden");$("app").inert=false;if(modalReturnFocus&&document.contains(modalReturnFocus))modalReturnFocus.focus();modalReturnFocus=null}
async function loadHistory(){if(!selectedId)return;try{const body=await api("/app/history");const rows=body.revisions||[];$("historyTimeline").innerHTML=rows.length?rows.map(r=>'<button class="history-item" data-rev="'+r.rev+'"><span>Revision '+r.rev+'</span><small>'+escapeHtml(r.change_count||0)+' changes'+(r.git_commit_short?" · "+escapeHtml(r.git_commit_short):"")+'</small></button>').join(""):'<div class="empty">No revisions yet.</div>';$("historyTimeline").querySelectorAll("[data-rev]").forEach(n=>n.onclick=()=>loadRevision(n.dataset.rev))}catch(err){$("historyTimeline").innerHTML='<div class="empty">'+escapeHtml(err.message)+'</div>'}}
async function loadRevision(rev){try{const body=await api("/app/history?rev="+encodeURIComponent(rev));const changes=body.changes||[];$("revisionSummary").textContent="Revision "+rev+" · "+(body.change_count||0)+" changes · Git "+(body.git_commit_short||"—");$("historyDetail").innerHTML=changes.length?changes.map(c=>'<div class="change-row"><strong>'+escapeHtml(c.op||"upsert")+' · '+escapeHtml(c.entity||"item")+'</strong><div>'+escapeHtml(c.rbx_path||c.new_rbx_path||c.local_path||"—")+'</div><div>'+escapeHtml(c.local_path||"")+'</div></div>').join(""):'<div class="empty">No detail.</div>'}catch(err){toast(err.message,"error")}}
function formatEntry(entry){const s=entry.source_summary||{};const title=s.first_line||String(entry.source||"").split(/\r?\n/).find(Boolean)||"(empty)";return{title,status:(entry.ok?"ok":entry.result_state||"error")+" · "+(entry.timeout_sec||0)+"s"}}
function renderExecHistory(){const list=$("execHistoryList");list.innerHTML=execHistory.length?execHistory.slice(0,30).map(entry=>{const f=formatEntry(entry);return '<div class="exec-history-item"><div class="exec-history-copy"><span class="exec-history-title" title="'+escapeHtml(f.title)+'">'+escapeHtml(f.title)+'</span><span class="exec-history-meta">'+escapeHtml(entry.submitted_by||"—")+' · '+escapeHtml(f.status)+'</span></div><button data-view="'+escapeHtml(entry.id)+'">View</button><button data-run="'+escapeHtml(entry.id)+'">Run</button></div>'}).join(""):'<div class="empty">No Remote Exec history yet.</div>';list.querySelectorAll("[data-view]").forEach(b=>b.onclick=()=>viewEntry(b.dataset.view));list.querySelectorAll("[data-run]").forEach(b=>b.onclick=()=>rerunExec(b.dataset.run))}
async function loadExecHistory(){if(!selectedId)return;try{const body=await api("/app/exec/history");execHistory=body.entries||[];$("execHistoryHint").textContent=body.warning||"Stored in this project's .rblxsync";renderExecHistory()}catch(err){$("execHistoryList").innerHTML='<div class="empty">'+escapeHtml(err.message)+'</div>'}}
function viewEntry(id){selectedEntry=execHistory.find(e=>e.id===id);if(!selectedEntry)return;const e=selectedEntry;const f=formatEntry(e);$("viewSubtitle").textContent=f.title;$("viewMeta").innerHTML=[["Status",e.result_state||"—"],["Submitted",e.submitted_by||"—"],["Timeout",(e.timeout_sec||0)+"s"],["Lines",e.source_summary?.line_count||0],["Duration",e.duration_ms?e.duration_ms+"ms":"—"],["Timestamp",e.timestamp?new Date(e.timestamp).toLocaleString():"—"]].map(x=>'<div class="meta-cell"><div class="label">'+escapeHtml(x[0])+'</div><div>'+escapeHtml(x[1])+'</div></div>').join("");$("viewSource").textContent=e.source||"";$("viewResult").textContent=e.output||e.traceback||e.error||"No stored output for this history entry.";openModal("viewModal")}
async function runExec(){const source=$("execSourceInput").value;if(!source.trim()){toast("Source is required.","warning");return}setExecBusy(true);try{const body=await api("/app/exec/run",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({source,timeout_sec:Number($("execTimeoutInput").value)||0})});renderExecResult(body);await loadExecHistory()}catch(err){$("execOutputText").textContent=err.message;$("execResultHint").textContent="Remote Exec failed";toast(err.message,"error")}finally{setExecBusy(false)}}
async function rerunExec(id){setExecBusy(true);try{const body=await api("/app/exec/rerun",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({id:id||"",timeout_sec:Number($("execTimeoutInput").value)||0})});renderExecResult(body);await loadExecHistory()}catch(err){$("execOutputText").textContent=err.message;$("execResultHint").textContent="Remote Exec failed"}finally{setExecBusy(false)}}
function renderExecResult(body){$("execOutputText").textContent=body.output||"";const r=body.result||{};$("execResultHint").textContent="state="+(r.state||"—")+" · ok="+!!r.ok+(r.duration_ms?" · "+r.duration_ms+"ms":"")}
function setExecBusy(value){busy=value;["execRunBtn","execLatestBtn","execHistoryRefreshBtn"].forEach(id=>$(id).disabled=value);$("execStatusText").textContent=value?"Running command...":"Start project and enable Exec in Studio."}
async function copyText(text){try{await navigator.clipboard.writeText(text);return true}catch(_){const a=document.createElement("textarea");a.value=text;a.style.position="fixed";a.style.left="-9999px";document.body.appendChild(a);a.select();let ok=false;try{ok=document.execCommand("copy")}catch(_){}a.remove();return ok}}
async function saveConfig(){if(!latest||latest.running||latest.starting||configApplying)return;configApplying=true;$("configApplyStatus").textContent="Saving...";try{const payload={config_path:$("configInput").value,sync_root:$("syncRootInput").value,host:$("hostInput").value,port:$("portInput").value,git_enabled:true,debug:$("debugInput").checked,legacy_scan:false};const body=await api("/app/config/apply",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});configDirty=false;configApplying=false;if(body.app)render(body.app);$("configApplyStatus").textContent="Saved";await loadInstances(false)}catch(err){configApplying=false;configDirty=true;$("configApplyStatus").textContent=err.message;toast(err.message,"error")}}
async function renameProject(){const name=$("instanceNameInput").value.trim();if(!name)return;try{await api("/app/instances/update",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({id:selectedId,name})});await loadInstances(true)}catch(err){toast(err.message,"error")}}
function requestRemoveProject(trigger=document.activeElement){const item=current();if(!item)return;if(latest?.running||latest?.starting){toast("Stop this project before removing it.","warning");return}pendingRemoveProjectId=item.id;$("removeProjectName").textContent=item.name;$("removeStatus").textContent="";$("removeKeepBtn").disabled=false;$("removeDeleteBtn").disabled=false;$("removeCancelBtn").disabled=false;openModal("removeModal",trigger)}
function closeRemoveModal(){pendingRemoveProjectId="";$("removeStatus").textContent="";closeModal("removeModal")}
async function confirmRemoveProject(deleteFiles){if(!pendingRemoveProjectId)return;const id=pendingRemoveProjectId;$("removeKeepBtn").disabled=true;$("removeDeleteBtn").disabled=true;$("removeCancelBtn").disabled=true;$("removeStatus").textContent=deleteFiles?"Deleting project files...":"Removing project...";try{const body=await api("/app/instances/remove",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({id,delete_files:!!deleteFiles})});pendingRemoveProjectId="";closeModal("removeModal");renderSidebar(body);await refreshSelected();toast(deleteFiles?"Project removed and files deleted.":"Project removed from RiftSync. Files were kept.","success")}catch(err){$("removeStatus").textContent=err.message;toast(err.message,"error")}finally{$("removeKeepBtn").disabled=false;$("removeDeleteBtn").disabled=false;$("removeCancelBtn").disabled=false}}
async function addProject(){const payload={name:$("addNameInput").value,sync_root:$("addRootInput").value,config_path:$("addConfigInput").value};$("addStatus").textContent="Adding...";try{const body=await api("/app/instances",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});renderSidebar(body);closeModal("addModal");$("addNameInput").value="";$("addRootInput").value="";$("addConfigInput").value="";await refreshSelected()}catch(err){$("addStatus").textContent=err.message}}
async function pickNewFolder(){try{const body=await api("/app/instances/pick-folder",{method:"POST"});if(body.selected)$("addRootInput").value=body.path}catch(err){toast(err.message,"error")}}
function markConfigDirty(){if(!latest||latest.running||latest.starting||configApplying)return;configDirty=true;$("configApplyStatus").textContent="Unsaved changes"}
async function pickConfigFolder(){if(latest&&(latest.running||latest.starting))return;try{const body=await api("/app/pick-folder",{method:"POST"});if(body.selected){$("syncRootInput").value=body.path;markConfigDirty()}}catch(err){toast(err.message,"error")}}
async function startAll(){busy=true;try{await api("/app/start-all",{method:"POST"});await loadInstances(true)}catch(err){toast(err.message,"error")}finally{busy=false}}
function formatLogs(body){const lines=["RiftSync project log","revision="+(body.revision||0)+" running="+!!body.running];if(body.git)lines.push("git="+(body.git.status||"-")+(body.git.last_commit_short?" @"+body.git.last_commit_short:"")+(body.git.last_error?" error="+body.git.last_error:""));lines.push("");(body.scan_warnings||[]).forEach(w=>lines.push("[warning] "+w));(body.events||[]).forEach(e=>lines.push("["+new Date((e.ts||0)*1000).toLocaleTimeString()+"] "+e.kind+"  "+e.message));return lines.join("\n")}
async function loadLogs(){try{$("logsText").textContent=formatLogs(await api("/app/logs"))}catch(err){$("logsText").textContent=err.message}}
["configInput","syncRootInput","hostInput","portInput"].forEach(id=>$(id).addEventListener("input",markConfigDirty));$("debugInput").addEventListener("change",markConfigDirty);
document.querySelectorAll(".tab").forEach(t=>{t.onclick=()=>setPanel(t.dataset.panel);t.onkeydown=e=>{const tabs=[...document.querySelectorAll(".tab")],index=tabs.indexOf(t);let next=index;if(e.key==="ArrowRight")next=(index+1)%tabs.length;else if(e.key==="ArrowLeft")next=(index-1+tabs.length)%tabs.length;else if(e.key==="Home")next=0;else if(e.key==="End")next=tabs.length-1;else return;e.preventDefault();setPanel(tabs[next].dataset.panel,true)}});
$("appHeader").onmousedown=e=>{if(!e.target.closest("button,input")&&window.appDragWindow)window.appDragWindow()};
$("minimizeBtn").onclick=e=>{e.stopPropagation();if(window.appMinimizeWindow)window.appMinimizeWindow()};
$("closeBtn").onclick=e=>{e.stopPropagation();fetch("/app/window/close",{method:"POST",keepalive:true}).catch(()=>{});if(window.appCloseWindow)window.appCloseWindow()};
const sidebarWrap=document.querySelector(".sidebar-wrap");
sidebarWrap.addEventListener("mouseleave",()=>sidebarWrap.classList.remove("force-collapsed"));sidebarWrap.addEventListener("focusin",()=>{if(!$("app").classList.contains("sidebar-pinned"))sidebarWrap.classList.remove("force-collapsed")});
let explorerTree=null,checkedPaths=new Set(),selectedNode=null,expandedPaths=new Set(),explorerFilter="",enabledServices=null,protecting=false,searchTimer=null;
const ENUM_OPTIONS={
  automaticsize:["None","X","Y","XY"],
  bordermode:["Outline","Inset","Middle"],
  scaletype:["Stretch","Slice","Tile","Fit","Crop"],
  zindexbehavior:["Sibling","Global"],
  textxalignment:["Left","Center","Right"],
  textyalignment:["Top","Center","Bottom"],
  runcontext:["Legacy","Server","Client","Plugin"],
  font:["Legacy","Arial","ArialBold","SourceSans","SourceSansBold","SourceSansItalic","SourceSansLight","SourceSansSemibold","Gotham","GothamBold","GothamMedium","GothamBlack"],
  applystrokemode:["Contextual","Border"],
  filldirection:["Horizontal","Vertical"],
  horizontalalignment:["Left","Center","Right"],
  verticalalignment:["Top","Center","Bottom"],
  sortorder:["LayoutOrder","Name","Custom"],
  material:["Plastic","SmoothPlastic","Neon","Wood","WoodPlanks","Marble","Slate","Concrete","Granite","Brick","Pebble","Cobblestone","Metal","DiamondPlate","Foil","Glass","ForceField"],
  ambientreverb:["NoReverb","GenericReverb","PaddedCell","Room","Bathroom","LivingRoom","StoneRoom","Auditorium","ConcertHall","Cave","Arena","Hangar","CarpettedHallway","Hallway","StoneCorridor","Alley","Forest","City","Mountains","Quarry","Plain","ParkingLot","SewerPipe","Underwater"],
  reverbtype:["NoReverb","GenericReverb","PaddedCell","Room","Bathroom","LivingRoom","StoneRoom","Auditorium","ConcertHall","Cave","Arena","Hangar","CarpettedHallway","Hallway","StoneCorridor","Alley","Forest","City","Mountains","Quarry","Plain","ParkingLot","SewerPipe","Underwater"],
  defaultlistenerlocation:["Camera","Character"],
  listenerlocation:["Camera","Character"],
  chatversion:["TextChatService","LegacyChatService"]
};
const CLASS_PROP_DEFAULTS={
  Frame:[
    {key:"AnchorPoint",type:"vector2",default:"0, 0"},
    {key:"AutomaticSize",type:"enum",default:"None"},
    {key:"BorderMode",type:"enum",default:"Outline"},
    {key:"BorderSizePixel",type:"number",default:0},
    {key:"LayoutOrder",type:"number",default:0},
    {key:"Visible",type:"bool",default:true},
    {key:"Active",type:"bool",default:false},
    {key:"ClipsDescendants",type:"bool",default:false},
    {key:"BackgroundColor3",type:"color",default:"#2d3748"},
    {key:"BackgroundTransparency",type:"number",default:0},
    {key:"Size",type:"udim2",default:"{0, 100}, {0, 100}"},
    {key:"Position",type:"udim2",default:"{0, 0}, {0, 0}"},
    {key:"ZIndex",type:"number",default:1}
  ],
  ScreenGui:[
    {key:"Enabled",type:"bool",default:true},
    {key:"ResetOnSpawn",type:"bool",default:true},
    {key:"DisplayOrder",type:"number",default:0},
    {key:"ZIndexBehavior",type:"enum",default:"Sibling"},
    {key:"IgnoreGuiInset",type:"bool",default:false}
  ],
  TextLabel:[
    {key:"AnchorPoint",type:"vector2",default:"0, 0"},
    {key:"AutomaticSize",type:"enum",default:"None"},
    {key:"Text",type:"text",default:"Label"},
    {key:"TextColor3",type:"color",default:"#ffffff"},
    {key:"TextSize",type:"number",default:14},
    {key:"TextScaled",type:"bool",default:false},
    {key:"TextWrapped",type:"bool",default:false},
    {key:"TextXAlignment",type:"enum",default:"Center"},
    {key:"TextYAlignment",type:"enum",default:"Center"},
    {key:"Font",type:"enum",default:"SourceSans"},
    {key:"Visible",type:"bool",default:true},
    {key:"BackgroundColor3",type:"color",default:"#1a202c"},
    {key:"BackgroundTransparency",type:"number",default:1},
    {key:"BorderSizePixel",type:"number",default:0},
    {key:"LayoutOrder",type:"number",default:0},
    {key:"Size",type:"udim2",default:"{0, 200}, {0, 50}"},
    {key:"Position",type:"udim2",default:"{0, 0}, {0, 0}"},
    {key:"ZIndex",type:"number",default:1}
  ],
  TextButton:[
    {key:"AnchorPoint",type:"vector2",default:"0, 0"},
    {key:"AutomaticSize",type:"enum",default:"None"},
    {key:"Text",type:"text",default:"Button"},
    {key:"TextColor3",type:"color",default:"#ffffff"},
    {key:"TextSize",type:"number",default:14},
    {key:"AutoButtonColor",type:"bool",default:true},
    {key:"TextXAlignment",type:"enum",default:"Center"},
    {key:"TextYAlignment",type:"enum",default:"Center"},
    {key:"Font",type:"enum",default:"SourceSans"},
    {key:"Visible",type:"bool",default:true},
    {key:"Active",type:"bool",default:true},
    {key:"BackgroundColor3",type:"color",default:"#2b6cb0"},
    {key:"BackgroundTransparency",type:"number",default:0},
    {key:"BorderSizePixel",type:"number",default:0},
    {key:"LayoutOrder",type:"number",default:0},
    {key:"Size",type:"udim2",default:"{0, 120}, {0, 40}"},
    {key:"Position",type:"udim2",default:"{0, 0}, {0, 0}"},
    {key:"ZIndex",type:"number",default:1}
  ],
  TextBox:[
    {key:"AnchorPoint",type:"vector2",default:"0, 0"},
    {key:"Text",type:"text",default:""},
    {key:"PlaceholderText",type:"text",default:""},
    {key:"TextColor3",type:"color",default:"#ffffff"},
    {key:"TextSize",type:"number",default:14},
    {key:"Cl"+"earTextOnFocus",type:"bool",default:true},
    {key:"MultiLine",type:"bool",default:false},
    {key:"Visible",type:"bool",default:true},
    {key:"BackgroundColor3",type:"color",default:"#1a202c"},
    {key:"BackgroundTransparency",type:"number",default:0},
    {key:"BorderSizePixel",type:"number",default:1},
    {key:"Size",type:"udim2",default:"{0, 200}, {0, 40}"},
    {key:"Position",type:"udim2",default:"{0, 0}, {0, 0}"}
  ],
  ImageLabel:[
    {key:"AnchorPoint",type:"vector2",default:"0, 0"},
    {key:"Image",type:"text",default:""},
    {key:"ImageColor3",type:"color",default:"#ffffff"},
    {key:"ImageTransparency",type:"number",default:0},
    {key:"ScaleType",type:"enum",default:"Stretch"},
    {key:"Visible",type:"bool",default:true},
    {key:"BackgroundTransparency",type:"number",default:1},
    {key:"Size",type:"udim2",default:"{0, 100}, {0, 100}"},
    {key:"Position",type:"udim2",default:"{0, 0}, {0, 0}"}
  ],
  ImageButton:[
    {key:"AnchorPoint",type:"vector2",default:"0, 0"},
    {key:"Image",type:"text",default:""},
    {key:"ImageColor3",type:"color",default:"#ffffff"},
    {key:"ScaleType",type:"enum",default:"Stretch"},
    {key:"AutoButtonColor",type:"bool",default:true},
    {key:"Visible",type:"bool",default:true},
    {key:"Active",type:"bool",default:true},
    {key:"BackgroundTransparency",type:"number",default:0},
    {key:"Size",type:"udim2",default:"{0, 100}, {0, 100}"},
    {key:"Position",type:"udim2",default:"{0, 0}, {0, 0}"}
  ],
  ScrollingFrame:[
    {key:"AnchorPoint",type:"vector2",default:"0, 0"},
    {key:"AutomaticCanvasSize",type:"enum",default:"None"},
    {key:"CanvasSize",type:"udim2",default:"{0, 0}, {2, 0}"},
    {key:"ScrollBarThickness",type:"number",default:6},
    {key:"ScrollBarImageColor3",type:"color",default:"#ffffff"},
    {key:"Visible",type:"bool",default:true},
    {key:"BackgroundColor3",type:"color",default:"#2d3748"},
    {key:"Size",type:"udim2",default:"{0, 200}, {0, 300}"},
    {key:"Position",type:"udim2",default:"{0, 0}, {0, 0}"}
  ],
  UICorner:[
    {key:"CornerRadius",type:"udim",default:"{0, 8}"}
  ],
  UIPadding:[
    {key:"PaddingTop",type:"udim",default:"{0, 0}"},
    {key:"PaddingBottom",type:"udim",default:"{0, 0}"},
    {key:"PaddingLeft",type:"udim",default:"{0, 0}"},
    {key:"PaddingRight",type:"udim",default:"{0, 0}"}
  ],
  UIStroke:[
    {key:"Color",type:"color",default:"#ffffff"},
    {key:"Thickness",type:"number",default:1},
    {key:"Transparency",type:"number",default:0},
    {key:"ApplyStrokeMode",type:"enum",default:"Contextual"}
  ],
  UIListLayout:[
    {key:"FillDirection",type:"enum",default:"Vertical"},
    {key:"HorizontalAlignment",type:"enum",default:"Left"},
    {key:"VerticalAlignment",type:"enum",default:"Top"},
    {key:"SortOrder",type:"enum",default:"LayoutOrder"},
    {key:"Padding",type:"udim",default:"{0, 0}"}
  ],
  UIGridLayout:[
    {key:"CellSize",type:"udim2",default:"{0, 100}, {0, 100}"},
    {key:"CellPadding",type:"udim2",default:"{0, 5}, {0, 5}"},
    {key:"SortOrder",type:"enum",default:"LayoutOrder"}
  ],
  Part:[
    {key:"Color",type:"color",default:"#a0a5a9"},
    {key:"Size",type:"vector3",default:"4, 1.2, 2"},
    {key:"Transparency",type:"number",default:0},
    {key:"CanCollide",type:"bool",default:true},
    {key:"Anchored",type:"bool",default:true},
    {key:"CastShadow",type:"bool",default:true},
    {key:"Material",type:"enum",default:"Plastic"}
  ],
  Model:[
    {key:"PrimaryPart",type:"text",default:""}
  ],
  Camera:[
    {key:"FieldOfView",type:"number",default:70}
  ],
  Sound:[
    {key:"SoundId",type:"text",default:""},
    {key:"Volume",type:"number",default:0.5},
    {key:"PlaybackSpeed",type:"number",default:1},
    {key:"Looped",type:"bool",default:false},
    {key:"Playing",type:"bool",default:false}
  ],
  Sky:[
    {key:"CelestialBodiesShown",type:"bool",default:true},
    {key:"MoonAngularSize",type:"number",default:11},
    {key:"MoonTextureId",type:"text",default:"rbxassetid://6444320592"},
    {key:"SkyboxBk",type:"text",default:""},
    {key:"SkyboxDn",type:"text",default:""},
    {key:"SkyboxFt",type:"text",default:""},
    {key:"SkyboxLf",type:"text",default:""},
    {key:"SkyboxRt",type:"text",default:""},
    {key:"SkyboxUp",type:"text",default:""},
    {key:"StarCount",type:"number",default:3000},
    {key:"SunAngularSize",type:"number",default:21},
    {key:"SunTextureId",type:"text",default:"rbxassetid://6444320592"}
  ],
  Atmosphere:[
    {key:"Density",type:"number",default:0.3},
    {key:"Offset",type:"number",default:0.25},
    {key:"Color",type:"color",default:"#c7c7c7"},
    {key:"Decay",type:"color",default:"#6a6a6a"},
    {key:"Glare",type:"number",default:0},
    {key:"Haze",type:"number",default:0}
  ],
  Lighting:[
    {key:"Ambient",type:"color",default:"#000000"},
    {key:"Brightness",type:"number",default:2},
    {key:"ColorShift_Bottom",type:"color",default:"#000000"},
    {key:"ColorShift_Top",type:"color",default:"#000000"},
    {key:"EnvironmentDiffuseScale",type:"number",default:1},
    {key:"EnvironmentSpecularScale",type:"number",default:1},
    {key:"GlobalShadows",type:"bool",default:true},
    {key:"OutdoorAmbient",type:"color",default:"#7f7f7f"},
    {key:"ShadowSoftness",type:"number",default:0.2},
    {key:"ClockTime",type:"number",default:14},
    {key:"GeographicLatitude",type:"number",default:41.73}
  ],
  Script:[
    {key:"Enabled",type:"bool",default:true},
    {key:"RunContext",type:"enum",default:"Legacy"}
  ],
  LocalScript:[
    {key:"Enabled",type:"bool",default:true}
  ],
  ModuleScript:[],
  SoundService:[
    {key:"AmbientReverb",type:"enum",default:"NoReverb"},
    {key:"DistanceFactor",type:"number",default:3.33},
    {key:"DopplerScale",type:"number",default:1},
    {key:"RolloffScale",type:"number",default:1},
    {key:"RespectFilteringEnabled",type:"bool",default:true},
    {key:"AcousticSimulationEnabled",type:"bool",default:false},
    {key:"DefaultListenerLocation",type:"enum",default:"Camera"}
  ],
  TextChatService:[
    {key:"ChatVersion",type:"enum",default:"TextChatService"}
  ]
};
const PROP_CAT_MAP={
  color:"Appearance",backgroundcolor3:"Appearance",textcolor3:"Appearance",bordercolor3:"Appearance",
  scrollbarimagecolor3:"Appearance",imagecolor3:"Appearance",placeholdercolor3:"Appearance",
  transparency:"Appearance",backgroundtransparency:"Appearance",texttransparency:"Appearance",
  imagetransparency:"Appearance",visible:"Appearance",skyboxbk:"Appearance",skyboxdn:"Appearance",
  skyboxft:"Appearance",skyboxlf:"Appearance",skyboxrt:"Appearance",skyboxup:"Appearance",
  moontextureid:"Appearance",suntextureid:"Appearance",ambient:"Appearance",outdoorambient:"Appearance",
  colorshift_top:"Appearance",colorshift_bottom:"Appearance",decay:"Appearance",glare:"Appearance",
  haze:"Appearance",castshadow:"Appearance",material:"Appearance",thickness:"Appearance",
  cancollide:"Behavior",anchored:"Behavior",enabled:"Behavior",active:"Behavior",
  clipsdescendants:"Behavior",resetonspawn:"Behavior",autobuttoncolor:"Behavior",scrollbarthickness:"Behavior",
  celestialbodiesshown:"Behavior",globalshadows:"Behavior",shadowsoftness:"Behavior",
  cleartextonfocus:"Behavior",multiline:"Behavior",ignoreguiinset:"Behavior",looped:"Behavior",
  playing:"Behavior",applystrokemode:"Behavior",respectfilteringenabled:"Behavior",acousticsimulationenabled:"Behavior",chatversion:"Behavior",
  anchorpoint:"Data",automaticsize:"Data",bordermode:"Data",layoutorder:"Data",zindex:"Data",
  zindexbehavior:"Data",size:"Data",position:"Data",canvassize:"Data",automaticcanvassize:"Data",
  text:"Data",font:"Data",textsize:"Data",textscaled:"Data",textwrapped:"Data",
  textxalignment:"Data",textyalignment:"Data",scaletype:"Data",image:"Data",displayorder:"Data",
  bordersizepixel:"Data",archivable:"Data",starcount:"Data",moonangularsize:"Data",sunangularsize:"Data",
  brightness:"Data",clocktime:"Data",geographiclatitude:"Data",density:"Data",offset:"Data",
  environmentdiffusescale:"Data",environmentspecularscale:"Data",value:"Data",cornerradius:"Data",
  paddingtop:"Data",paddingbottom:"Data",paddingleft:"Data",paddingright:"Data",padding:"Data",
  cellsize:"Data",cellpadding:"Data",filldirection:"Data",horizontalalignment:"Data",
  verticalalignment:"Data",sortorder:"Data",fieldofview:"Data",volume:"Data",playbackspeed:"Data",
  soundid:"Data",runcontext:"Data",primarypart:"Data",ambientreverb:"Data",reverbtype:"Data",
  distancefactor:"Data",dopplerscale:"Data",rolloffscale:"Data",defaultlistenerlocation:"Data",listenerlocation:"Data"
};
function getPropCategory(key){return PROP_CAT_MAP[key.toLowerCase()]||"Data"}
function colorToHex(val){
  if(!val)return "#FFFFFF";
  if(typeof val==="string"){
    if(/^#[0-9a-f]{6}$/i.test(val))return val.toUpperCase();
    return "#FFFFFF";
  }
  let r=255,g=255,b=255;
  if(Array.isArray(val)&&val.length>=3){
    r=val[0];g=val[1];b=val[2];
  }else if(typeof val==="object"){
    r=val.r??val.R??255;
    g=val.g??val.G??255;
    b=val.b??val.B??255;
  }
  if(r<=1&&g<=1&&b<=1&&(r>0||g>0||b>0)){
    r=Math.round(r*255);g=Math.round(g*255);b=Math.round(b*255);
  }
  const to2=n=>Math.max(0,Math.min(255,Math.round(Number(n)||0))).toString(16).padStart(2,"0").toUpperCase();
  return "#"+to2(r)+to2(g)+to2(b);
}
function formatUDim2Display(val){
  if(!val)return "{0, 0}, {0, 0}";
  if(typeof val==="string")return val;
  if(Array.isArray(val)&&val.length>=4){
    return "{"+val[0]+", "+val[1]+"}, {"+val[2]+", "+val[3]+"}";
  }
  if(typeof val==="object"){
    const xs=val.xScale??val.XScale??0,xo=val.xOffset??val.XOffset??0,ys=val.yScale??val.YScale??0,yo=val.yOffset??val.YOffset??0;
    return "{"+xs+", "+xo+"}, {"+ys+", "+yo+"}";
  }
  return String(val);
}
function parseUDim2String(text){
  if(!text||typeof text!=="string")return null;
  const m=text.match(/\{?\s*(-?[\d\.]+)\s*,\s*(-?[\d\.]+)\s*\}?\s*,\s*\{?\s*(-?[\d\.]+)\s*,\s*(-?[\d\.]+)\s*\}?/);
  if(m){
    return {"$type":"UDim2","xScale":parseFloat(m[1])||0,"xOffset":parseFloat(m[2])||0,"yScale":parseFloat(m[3])||0,"yOffset":parseFloat(m[4])||0};
  }
  return null;
}
function formatUDimDisplay(val){
  if(!val)return "{0, 0}";
  if(typeof val==="string")return val;
  if(Array.isArray(val)&&val.length>=2)return "{"+val[0]+", "+val[1]+"}";
  if(typeof val==="object"){
    const s=val.scale??val.Scale??0,o=val.offset??val.Offset??0;
    return "{"+s+", "+o+"}";
  }
  return String(val);
}
function parseUDimString(text){
  if(!text||typeof text!=="string")return null;
  const m=text.match(/\{?\s*(-?[\d\.]+)\s*,\s*(-?[\d\.]+)\s*\}?/);
  if(m){
    return {"$type":"UDim","scale":parseFloat(m[1])||0,"offset":parseFloat(m[2])||0};
  }
  return null;
}
function formatVector2Display(val){
  if(!val)return "0, 0";
  if(typeof val==="string")return val;
  if(Array.isArray(val)&&val.length>=2)return val[0]+", "+val[1];
  if(typeof val==="object"){
    const x=val.x??val.X??0,y=val.y??val.Y??0;
    return x+", "+y;
  }
  return String(val);
}
function parseVector2String(text){
  if(!text||typeof text!=="string")return null;
  const m=text.match(/(-?[\d\.]+)\s*,\s*(-?[\d\.]+)/);
  if(m){
    return {"$type":"Vector2","x":parseFloat(m[1])||0,"y":parseFloat(m[2])||0};
  }
  return null;
}
function formatVector3Display(val){
  if(!val)return "0, 0, 0";
  if(typeof val==="string")return val;
  if(Array.isArray(val)&&val.length>=3)return val[0]+", "+val[1]+", "+val[2];
  if(typeof val==="object"){
    const x=val.x??val.X??0,y=val.y??val.Y??0,z=val.z??val.Z??0;
    return x+", "+y+", "+z;
  }
  return String(val);
}
function parseVector3String(text){
  if(!text||typeof text!=="string")return null;
  const m=text.match(/(-?[\d\.]+)\s*,\s*(-?[\d\.]+)\s*,\s*(-?[\d\.]+)/);
  if(m){
    return {"$type":"Vector3","x":parseFloat(m[1])||0,"y":parseFloat(m[2])||0,"z":parseFloat(m[3])||0};
  }
  return null;
}
function formatEnumDisplay(val){
  if(!val)return "";
  if(typeof val==="string")return val;
  if(typeof val==="object")return val.name??val.value??val.Name??val.Value??"";
  return String(val);
}
function getRobloxIcon(kind,name){
  const k=String(kind||"").toLowerCase(),n=String(name||"").toLowerCase();
  if(n==="workspace"||k==="workspace")return '<svg viewBox="0 0 16 16" width="16" height="16"><circle cx="8" cy="8" r="7" fill="#2d6cb5"/><path d="M8 1a7 7 0 0 0 0 14A7 7 0 0 0 8 1zm0 2c1.2 0 2.2 2.2 2.2 5S9.2 13 8 13s-2.2-2.2-2.2-5S6.8 3 8 3zM2 8c0-.6.1-1.2.3-1.8h2.3c-.1.6-.1 1.2-.1 1.8s0 1.2.1 1.8H2.3A6.9 6.9 0 0 1 2 8zm9.1 1.8c.1-.6.1-1.2.1-1.8s0-1.2-.1-1.8h2.3c.2.6.3 1.2.3 1.8s-.1 1.2-.3 1.8h-2.3z" fill="#fff"/></svg>';
  if(n==="serverscriptservice"||k==="serverscriptservice")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#1b4d3e"/><path d="M4 3h8v2H4V3zm0 3h8v2H4V6zm0 3h5v2H4V9z" fill="#66dc93"/></svg>';
  if(n==="replicatedstorage"||k==="replicatedstorage")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#3a4b63"/><path d="M8 2L2 5.5 8 9l6-3.5L8 2zm-5 5.2v3.6L8 14v-3.6L3 7.2zm10 0L8 10.4V14l5-3.2V7.2z" fill="#829bf0"/></svg>';
  if(n==="serverstorage"||k==="serverstorage")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#4a3e2a"/><path d="M3 3h10v3H3V3zm0 4h10v3H3V7zm0 4h10v3H3v-3z" fill="#f0c36d"/></svg>';
  if(n==="startergui"||k==="startergui")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#2d5568"/><rect x="3" y="3" width="10" height="7" rx="1" fill="#68d7d0"/><rect x="5" y="11" width="6" height="2" fill="#68d7d0"/></svg>';
  if(n==="starterpack"||k==="starterpack")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#543825"/><path d="M5 3h6v3H5V3zm-2 4h10v7H3V7zm2 2v3h6V9H5z" fill="#f0a068"/></svg>';
  if(n.startsWith("starterplayer")||k.startsWith("starterplayer"))return '<svg viewBox="0 0 16 16" width="16" height="16"><circle cx="8" cy="5" r="3" fill="#e0e0e0"/><path d="M3 14c0-2.8 2.2-5 5-5s5 2.2 5 5H3z" fill="#e0e0e0"/></svg>';
  if(n==="soundservice"||k==="soundservice")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#3a3020"/><path d="M3 6h2.5l3.5-3v10l-3.5-3H3V6z" fill="#f0c36d"/><path d="M11 5.5a3.5 3.5 0 0 1 0 5" stroke="#f0c36d" stroke-width="1.3" fill="none"/><path d="M12.8 3.8a6 6 0 0 1 0 8.4" stroke="#f0c36d" stroke-width="1.1" fill="none"/></svg>';
  if(n==="lighting"||k==="lighting")return '<svg viewBox="0 0 16 16" width="16" height="16"><circle cx="8" cy="8" r="3.5" fill="#f0c36d"/><path d="M8 1v2m0 10v2M1 8h2m10 0h2m-2.5-4.5l-1.5 1.5m-7 7l-1.5 1.5m10 0l-1.5-1.5m-7-7l-1.5-1.5" stroke="#f0c36d" stroke-width="1.2"/></svg>';
  if(n==="textchatservice"||k==="textchatservice")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#203d4a"/><path d="M3 4h10v6H6l-3 3V4z" fill="#68d7d0"/></svg>';
  if(n==="replicatedfirst"||k==="replicatedfirst")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#4a2222"/><path d="M4 3l5 5-5 5V3zm5 0l5 5-5 5V3z" fill="#f08080"/></svg>';
  if(k==="service")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#34495e"/><circle cx="8" cy="8" r="4" fill="#aeb9c8"/></svg>';

  // Effects & post-processing
  if(k.includes("bloom")||n.includes("bloom"))return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#383018"/><path d="M8 2l1.5 4.5L14 8l-4.5 1.5L8 14l-1.5-4.5L2 8l4.5-1.5L8 2z" fill="#f0d068"/></svg>';
  if(k.includes("colorcorrection")||n.includes("colorcorrection"))return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#321e3c"/><circle cx="6" cy="8" r="4" fill="#d084f7" fill-opacity="0.7"/><circle cx="10" cy="8" r="4" fill="#68d7d0" fill-opacity="0.7"/></svg>';
  if(k.includes("depthoffield")||n.includes("depthoffield"))return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#183038"/><circle cx="8" cy="8" r="5" fill="none" stroke="#68d7d0" stroke-width="1.5"/><circle cx="8" cy="8" r="2" fill="#68d7d0"/></svg>';
  if(k.includes("sunrays")||n.includes("sunrays"))return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#3a3014"/><circle cx="8" cy="8" r="3" fill="#f0c36d"/><path d="M8 1v2m0 10v2M1 8h2m10 0h2m-2-4.5l1.5-1.5m-9 9l1.5-1.5m6 0l1.5 1.5m-9-9l1.5 1.5" stroke="#f0c36d" stroke-width="1.3"/></svg>';
  if(k.includes("blur")||n.includes("blur"))return '<svg viewBox="0 0 16 16" width="16" height="16"><rect width="16" height="16" rx="3" fill="#202a3a"/><circle cx="8" cy="8" r="5" fill="#829bf0" fill-opacity="0.5"/><circle cx="8" cy="8" r="2.5" fill="#829bf0"/></svg>';

  // Standard Instances
  if(k==="sky")return '<svg viewBox="0 0 16 16" width="16" height="16"><circle cx="8" cy="8" r="6" fill="#1b4d7e"/><path d="M4 11a4 4 0 0 1 8 0H4z" fill="#f0c36d"/><circle cx="10" cy="5" r="1.5" fill="#fff"/></svg>';
  if(k==="atmosphere")return '<svg viewBox="0 0 16 16" width="16" height="16"><circle cx="8" cy="8" r="6" fill="none" stroke="#68d7d0" stroke-width="1.5"/><circle cx="8" cy="8" r="3.5" fill="#204050"/></svg>';
  if(k==="screengui")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="1" y="2" width="14" height="10" rx="1.5" fill="#25384d" stroke="#5cb8ff" stroke-width="1.2"/><rect x="5" y="13" width="6" height="1.5" rx="0.5" fill="#5cb8ff"/></svg>';
  if(k==="frame")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="2" width="12" height="12" rx="1.5" fill="#2a3b4c" stroke="#68d7d0" stroke-width="1.2"/></svg>';
  if(k==="scrollingframe")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="2" width="12" height="12" rx="1.5" fill="#2a3b4c" stroke="#68d7d0" stroke-width="1"/><rect x="11" y="4" width="2" height="5" rx="0.5" fill="#68d7d0"/></svg>';
  if(k==="textlabel"||k==="textbox")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="2" width="12" height="12" rx="1.5" fill="#1e3428"/><text x="8" y="11.5" font-size="10" font-family="sans-serif" font-weight="bold" fill="#66dc93" text-anchor="middle">T</text></svg>';
  if(k==="textbutton")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="1.5" y="3" width="13" height="10" rx="2" fill="#2d5568" stroke="#68d7d0" stroke-width="1"/><text x="8" y="10.5" font-size="8" font-family="sans-serif" font-weight="bold" fill="#fff" text-anchor="middle">OK</text></svg>';
  if(k==="imagelabel")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="2" width="12" height="12" rx="1.5" fill="#3a2f4c"/><circle cx="5.5" cy="5.5" r="1.5" fill="#d084f7"/><path d="M3 12l3-3.5 2 2 3-4 2 2.5v3H3z" fill="#d084f7"/></svg>';
  if(k==="imagebutton")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="1.5" y="2" width="13" height="12" rx="2" fill="#3a2f4c" stroke="#d084f7" stroke-width="1"/><circle cx="5" cy="5" r="1.2" fill="#d084f7"/><path d="M3 11l2.5-3 2 2 2.5-3.5 2 2v2.5H3z" fill="#d084f7"/></svg>';
  if(k.includes("uilayout")||k==="uicorner"||k==="uipadding"||k==="uiscale"||k==="uigradient"||k==="uistroke")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="2" width="5" height="5" rx="1" fill="#f0c36d"/><rect x="9" y="2" width="5" height="5" rx="1" fill="#f0c36d"/><rect x="2" y="9" width="5" height="5" rx="1" fill="#f0c36d"/><rect x="9" y="9" width="5" height="5" rx="1" fill="#f0c36d"/></svg>';
  if(k==="model")return '<svg viewBox="0 0 16 16" width="16" height="16"><path d="M8 1.5L14 5v6l-6 3.5L2 11V5l6-3.5z" fill="#2d5272" stroke="#5cb8ff" stroke-width="1.2"/><path d="M8 1.5v13M2 5l6 3.5 6-3.5" stroke="#5cb8ff" stroke-width="1"/></svg>';
  if(k==="part"||k==="meshpart"||k==="unionoperation")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2.5" y="4" width="11" height="8" rx="1.5" fill="#6d7d91" stroke="#9bb1c9" stroke-width="1"/><circle cx="5.5" cy="5.5" r="1" fill="#c3d2e0"/><circle cx="10.5" cy="5.5" r="1" fill="#c3d2e0"/></svg>';
  if(k==="configuration")return '<svg viewBox="0 0 16 16" width="16" height="16"><circle cx="8" cy="8" r="3" fill="none" stroke="#aeb9c8" stroke-width="2"/><path d="M8 1v2m0 10v2M1 8h2m10 0h2m-2.5-4.5l-1.5 1.5m-7 7l-1.5 1.5m10 0l-1.5-1.5m-7-7l-1.5-1.5" stroke="#aeb9c8" stroke-width="1.5"/></svg>';
  if(k==="remoteevent")return '<svg viewBox="0 0 16 16" width="16" height="16"><path d="M9 1L4 9h4l-1 6 6-8H9l1-6z" fill="#e0c56e"/></svg>';
  if(k==="remotefunction")return '<svg viewBox="0 0 16 16" width="16" height="16"><path d="M9 1L4 8h4l-1 5 6-7H9l1-5z" fill="#d084f7"/><path d="M3 13h10" stroke="#d084f7" stroke-width="1.5"/></svg>';
  if(k.includes("value"))return '<svg viewBox="0 0 16 16" width="16" height="16"><circle cx="8" cy="8" r="6" fill="#32475e" stroke="#68d7d0" stroke-width="1.2"/><text x="8" y="11" font-size="8" font-family="sans-serif" font-weight="bold" fill="#68d7d0" text-anchor="middle">V</text></svg>';
  if(k==="sound")return '<svg viewBox="0 0 16 16" width="16" height="16"><path d="M3 6h3l4-3v10l-4-3H3V6z" fill="#f0c36d"/><path d="M12 5a4 4 0 0 1 0 6" stroke="#f0c36d" stroke-width="1.5" fill="none"/></svg>';
  if(k==="camera")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="4" width="9" height="8" rx="1.5" fill="#3a4b63"/><polygon points="11,6 15,4 15,12 11,10" fill="#5cb8ff"/></svg>';
  if(k==="folder")return '<svg viewBox="0 0 16 16" width="16" height="16"><path d="M2 3h4l1.5 2H14v8H2V3z" fill="#e0c56e"/></svg>';
  if(k==="server_script"||k==="script")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="1" width="12" height="14" rx="2" fill="#234e70"/><circle cx="11" cy="11" r="3" fill="#66dc93"/><path d="M4 4h8v1.5H4V4zm0 3h8v1.5H4V7zm0 3h4v1.5H4V10z" fill="#fff"/></svg>';
  if(k==="client_script"||k==="localscript")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="1" width="12" height="14" rx="2" fill="#234e70"/><circle cx="11" cy="11" r="3" fill="#829bf0"/><path d="M4 4h8v1.5H4V4zm0 3h8v1.5H4V7zm0 3h4v1.5H4V10z" fill="#fff"/></svg>';
  if(k==="module_script"||k==="modulescript")return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="2" y="1" width="12" height="14" rx="2" fill="#20344d"/><path d="M7 3h2v2H7V3zm-3 3h3v2H4V6zm5 0h3v2H9V6zm-2 3h2v2H7V9zm-3 3h3v2H4v-2zm5 0h3v2H9v-2z" fill="#68d7d0"/></svg>';
  return '<svg viewBox="0 0 16 16" width="16" height="16"><rect x="3" y="2" width="10" height="12" rx="1.5" fill="#324458" stroke="#48627e" stroke-width="1"/><path d="M5 5h6M5 8h6M5 11h4" stroke="#7a97b8" stroke-width="1.2"/></svg>';
}
function setSubTab(tab){
  const isProps=tab==="props";
  $("subTabProps").classList.toggle("active",isProps);
  $("subTabSecurity").classList.toggle("active",!isProps);
  $("propsSubPanel").hidden=!isProps;
  $("securitySubPanel").hidden=isProps;
}
function collectNodeScripts(node,out=[]){
  if(!node)return out;
  if(node.is_script)out.push(node.script_rel_path||node.path);
  if(node.children){for(const c of node.children)collectNodeScripts(c,out)}
  return out;
}
function collectAllNodePaths(node,out=[]){
  if(!node)return out;
  if(node.path)out.push(node.path);
  if(node.children){for(const c of node.children)collectAllNodePaths(c,out)}
  return out;
}
function findNodeByPath(node,path){
  if(!node)return null;
  if(node.path===path)return node;
  if(node.children){for(const c of node.children){const res=findNodeByPath(c,path);if(res)return res}}
  return null;
}
function isRootService(node){
  if(!node||!node.path)return true;
  const parts=node.path.split("/");
  return parts.length===1&&(["workspace","replicatedstorage","serverscriptservice","serverstorage","startergui","starterplayer","starterpack","lighting","soundservice","textchatservice","replicatedfirst","voicechatservice","localizationservice","teams"].includes(parts[0].toLowerCase())||node.kind==="service");
}
function renderPropControlHTML(key,type,val){
  const lk=key.toLowerCase();
  const isEnum=type==="enum"||(val&&val.$type==="Enum")||ENUM_OPTIONS[lk];
  if(isEnum){
    const curVal=formatEnumDisplay(val);
    const options=ENUM_OPTIONS[lk]||[];
    if(options.length>0){
      let optsHtml="";
      let found=false;
      options.forEach(opt=>{
        const sel=opt.toLowerCase()===curVal.toLowerCase()?" selected":"";
        if(sel)found=true;
        optsHtml+='<option value="'+escapeHtml(opt)+'"'+sel+'>'+escapeHtml(opt)+'</option>';
      });
      if(!found&&curVal){
        optsHtml='<option value="'+escapeHtml(curVal)+'" selected>'+escapeHtml(curVal)+'</option>'+optsHtml;
      }
      return '<select class="prop-enum-select" data-enum-type="'+escapeHtml(val?.enumType||key)+'">'+optsHtml+'</select>';
    }
    return '<input type="text" class="prop-text-input" value="'+escapeHtml(curVal)+'" spellcheck="false">';
  }else if(type==="bool"){
    const isChecked=val===true;
    return '<input type="checkbox" class="prop-checkbox" '+(isChecked?"checked":"")+'>';
  }else if(type==="color"){
    const hex=colorToHex(val);
    return '<div class="prop-color-row">'+
      '<input type="color" class="prop-color-swatch" value="'+escapeHtml(hex)+'">'+
      '<input type="text" class="prop-color-input" value="'+escapeHtml(hex)+'" spellcheck="false" style="font-family:monospace;height:20px;font-size:11px">'+
    '</div>';
  }else if(type==="vector2"){
    return '<input type="text" class="prop-text-input" value="'+escapeHtml(formatVector2Display(val))+'" placeholder="0, 0" spellcheck="false">';
  }else if(type==="vector3"){
    return '<input type="text" class="prop-text-input" value="'+escapeHtml(formatVector3Display(val))+'" placeholder="0, 0, 0" spellcheck="false">';
  }else if(type==="udim"){
    return '<input type="text" class="prop-text-input" value="'+escapeHtml(formatUDimDisplay(val))+'" placeholder="{0, 0}" spellcheck="false">';
  }else if(type==="udim2"){
    return '<input type="text" class="prop-text-input" value="'+escapeHtml(formatUDim2Display(val))+'" placeholder="{0, 0}, {0, 0}" spellcheck="false">';
  }else if(type==="number"){
    return '<input type="number" class="prop-num-input" value="'+(Number(val)||0)+'" step="any">';
  }else{
    const displayVal=(val&&typeof val==="object")?(val.name??val.value??val.family??JSON.stringify(val)):String(val??"");
    return '<input type="text" class="prop-text-input" value="'+escapeHtml(displayVal)+'" spellcheck="false">';
  }
}
function getPropTooltip(key,type){
  const lk=key.toLowerCase();
  if(type==="udim2"||lk==="position"||lk==="size"||lk==="canvassize"||lk==="cellsize"||lk==="cellpadding")return key+" (UDim2): {X.Scale, X.Offset}, {Y.Scale, Y.Offset}";
  if(type==="vector2"||lk.includes("anchorpoint")||lk==="canvasposition")return key+" (Vector2): X, Y";
  if(type==="vector3")return key+" (Vector3): X, Y, Z";
  if(type==="udim"||lk==="cornerradius"||lk.startsWith("padding"))return key+" (UDim): Scale, Offset";
  if(type==="color"||lk.includes("color"))return key+" (Color3): [Red, Green, Blue] / #Hex";
  if(type==="enum"||ENUM_OPTIONS[lk]){const opts=ENUM_OPTIONS[lk]||[];return key+" (Enum): "+(opts.length?opts.join(", "):"Options")}
  if(type==="bool"||["visible","enabled","anchored","cancollide","active","archivable","clipsdescendants","resetonspawn","multiline"].includes(lk)||lk.includes("cleartextonfocus"))return key+" (boolean): true / false";
  if(type==="number"||lk.includes("transparency")||lk==="zindex"||lk==="layoutorder"||lk==="bordersizepixel"||lk==="displayorder"){
    if(lk.includes("transparency"))return key+" (float): 0.0 (opaque) to 1.0 (invisible)";
    if(lk==="zindex"||lk==="layoutorder"||lk==="bordersizepixel"||lk==="displayorder")return key+" (integer)";
    return key+" (number)";
  }
  return key+" ("+(type||"string")+")";
}
function renderPropRowHTML(key,type,val,isCustom){
  let nameHtml="";
  const tooltip=getPropTooltip(key,type);
  if(!isCustom){
    nameHtml='<div class="prop-name-cell" title="'+escapeHtml(tooltip)+'">'+escapeHtml(key)+'</div>';
  }else{
    nameHtml='<div class="prop-name-cell" title="Custom Attribute" style="padding:1px 4px">'+
      '<div style="display:flex;align-items:center;gap:3px;width:100%">'+
        '<input type="text" class="prop-attr-key-input" value="'+escapeHtml(key)+'" placeholder="Name" spellcheck="false">'+
        '<button class="prop-del-btn" title="Remove attribute">×</button>'+
      '</div>'+
    '</div>';
  }
  return '<div class="prop-row" data-key="'+escapeHtml(key)+'" data-type="'+type+'" data-custom="'+(isCustom?"1":"0")+'" title="'+escapeHtml(tooltip)+'">'+
    nameHtml+
    '<div class="prop-val-cell">'+renderPropControlHTML(key,type,val)+'</div>'+
  '</div>';
}
function addCustomPropRow(){
  const list=$("propAttrList");
  if(!list)return;
  const row=document.createElement("div");
  row.className="prop-row";
  row.dataset.type="text";
  row.dataset.custom="1";
  row.innerHTML='<div class="prop-name-cell" style="padding:1px 4px">'+
    '<div style="display:flex;align-items:center;gap:3px;width:100%">'+
      '<input type="text" class="prop-attr-key-input" placeholder="Name" spellcheck="false">'+
      '<button class="prop-del-btn" title="Remove attribute">×</button>'+
    '</div>'+
  '</div>'+
  '<div class="prop-val-cell"><input type="text" class="prop-text-input" placeholder="Value" spellcheck="false"></div>';
  row.querySelector(".prop-del-btn").onclick=()=>row.remove();
  list.appendChild(row);
  row.querySelector(".prop-attr-key-input").focus();
}
function renderPropertiesForm(node){
  const form=$("propertiesForm");
  if(!node){
    form.innerHTML='<div class="hint" style="text-align:center;padding:24px 0">Select an item in Explorer to view properties.</div>';
    return;
  }
  const cls=node.class_name||(node.is_script?(node.kind==="client_script"?"LocalScript":node.kind==="module_script"?"ModuleScript":"Script"):"Folder");
  const defaults=CLASS_PROP_DEFAULTS[cls]||[];
  const existingProps=Object.assign({},node.properties||{});
  const propMap=new Map();
  defaults.forEach(d=>{
    const val=existingProps[d.key]!==undefined?existingProps[d.key]:d.default;
    propMap.set(d.key,{key:d.key,type:d.type,val:val});
    delete existingProps[d.key];
  });
  Object.keys(existingProps).forEach(k=>{
    const val=existingProps[k];
    let type="text";
    const lk=k.toLowerCase();
    if(val&&val.$type==="Vector2")type="vector2";
    else if(val&&val.$type==="Vector3")type="vector3";
    else if(val&&val.$type==="Enum")type="enum";
    else if(val&&val.$type==="UDim")type="udim";
    else if(val&&val.$type==="UDim2")type="udim2";
    else if(val&&val.$type==="Color3")type="color";
    else if(ENUM_OPTIONS[lk])type="enum";
    else if(lk.includes("anchorpoint"))type="vector2";
    else if(lk==="cornerradius"||lk.startsWith("padding"))type="udim";
    else if(lk.includes("color")||(typeof val==="string"&&/^#[0-9a-f]{6}$/i.test(val))){
      type="color";
    }else if(typeof val==="boolean"||["enabled","visible","resetonspawn","cancollide","anchored","active","clipsdescendants","textscaled","textwrapped","celestialbodiesshown","globalshadows","castshadow","cleartextonfocus","multiline","ignoreguiinset"].includes(lk)){
      type="bool";
    }else if(lk==="size"||lk==="position"||lk==="canvassize"||lk==="cellsize"||lk==="cellpadding"){
      type="udim2";
    }else if(typeof val==="number"||["zindex","textsize","displayorder","bordersizepixel","backgroundtransparency","transparency","starcount","moonangularsize","sunangularsize","brightness","clocktime","geographiclatitude","density","offset","glare","haze","layoutorder","thickness","volume","playbackspeed","fieldofview"].includes(lk)){
      type="number";
    }
    propMap.set(k,{key:k,type:type,val:val});
  });
  const categories={Appearance:[],Data:[],Behavior:[]};
  for(const [key,item] of propMap.entries()){
    const cat=getPropCategory(key);
    if(!categories[cat])categories[cat]=[];
    categories[cat].push(item);
  }
  let html="";
  const catOrder=["Appearance","Data","Behavior"];
  catOrder.forEach(catName=>{
    const items=categories[catName];
    if(!items||!items.length)return;
    items.sort((a,b)=>a.key.localeCompare(b.key));
    html+='<div class="prop-category"><span>'+catName+'</span></div>';
    items.forEach(item=>{
      html+=renderPropRowHTML(item.key,item.type,item.val,false);
    });
  });
  html+='<div class="prop-category" style="justify-content:space-between">'+
    '<span>Attributes</span>'+
    '<button id="addAttrBtn" class="optional" style="font-size:10px;padding:1px 6px;height:18px;line-height:1;margin:0" title="Add custom attribute">+ Add</button>'+
  '</div>'+
  '<div id="propAttrList"></div>';
  form.innerHTML=html;
  form.querySelectorAll(".prop-row[data-type='color']").forEach(row=>{
    const picker=row.querySelector(".prop-color-swatch");
    const hexInput=row.querySelector(".prop-color-input");
    if(picker&&hexInput){
      picker.addEventListener("input",e=>{hexInput.value=e.target.value.toUpperCase()});
      hexInput.addEventListener("input",e=>{if(/^#[0-9a-f]{6}$/i.test(e.target.value))picker.value=e.target.value});
    }
  });
  const addBtn=$("addAttrBtn");
  if(addBtn)addBtn.onclick=()=>addCustomPropRow();
}
async function saveProperties(){
  if(!selectedNode||!selectedNode.path)return;
  const btn=$("savePropsBtn");
  btn.disabled=true;
  btn.textContent="Saving...";
  try{
    const newProps={};
    const rows=document.querySelectorAll("#propertiesForm .prop-row");
    rows.forEach(row=>{
      const isCustom=row.dataset.custom==="1";
      const key=isCustom?(row.querySelector(".prop-attr-key-input")?.value?.trim()||""):row.dataset.key;
      if(!key)return;
      const type=row.dataset.type;
      if(type==="bool"){
        const chk=row.querySelector(".prop-checkbox");
        newProps[key]=!!chk.checked;
      }else if(type==="enum"){
        const sel=row.querySelector(".prop-enum-select");
        const textInp=row.querySelector(".prop-text-input");
        const enumVal=sel?sel.value:(textInp?textInp.value.trim():"");
        const enumType=sel?.dataset?.enumType||key;
        newProps[key]={"$type":"Enum","enumType":enumType,"value":enumVal};
      }else if(type==="vector2"){
        const textInp=row.querySelector(".prop-text-input");
        const parsed=parseVector2String(textInp.value);
        newProps[key]=parsed||textInp.value;
      }else if(type==="vector3"){
        const textInp=row.querySelector(".prop-text-input");
        const parsed=parseVector3String(textInp.value);
        newProps[key]=parsed||textInp.value;
      }else if(type==="udim"){
        const textInp=row.querySelector(".prop-text-input");
        const parsed=parseUDimString(textInp.value);
        newProps[key]=parsed||textInp.value;
      }else if(type==="udim2"){
        const textInp=row.querySelector(".prop-text-input");
        const parsed=parseUDim2String(textInp.value);
        newProps[key]=parsed||textInp.value;
      }else if(type==="color"){
        const colorInp=row.querySelector(".prop-color-input");
        const hex=colorInp?colorInp.value:"#FFFFFF";
        const r=parseInt(hex.slice(1,3),16)||0;
        const g=parseInt(hex.slice(3,5),16)||0;
        const b=parseInt(hex.slice(5,7),16)||0;
        newProps[key]={"$type":"Color3","r":r,"g":g,"b":b};
      }else if(type==="number"){
        const numInp=row.querySelector(".prop-num-input");
        newProps[key]=Number(numInp.value)||0;
      }else{
        const textInp=row.querySelector(".prop-text-input");
        const val=textInp?textInp.value:"";
        try{
          if((val.startsWith("{")&&val.endsWith("}"))||(val.startsWith("[")&&val.endsWith("]"))){
            newProps[key]=JSON.parse(val);
          }else{
            newProps[key]=val;
          }
        }catch(_){
          newProps[key]=val;
        }
      }
    });
    await api("/app/explorer/set-properties",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({path:selectedNode.path,properties:newProps})});
    toast("Properties saved for "+selectedNode.name,"success");
    await loadExplorerTree();
  }catch(err){
    toast("Save properties failed: "+err.message,"error");
  }finally{
    btn.disabled=false;
    btn.textContent="Save Properties";
  }
}
function updateSelectedNodeDisplay(){
  const info=$("selectedNodeInfo"),pill=$("selectedNodeClassPill"),icon=$("selectedNodeIcon"),p=$("selectedNodePath"),badges=$("selectedNodeBadges"),scriptRow=$("scriptActionRow");
  const bT=$("toggleSingleEnabledBtn"),bSave=$("savePropsBtn"),bRen=$("renameNodeBtn");
  if(!selectedNode){
    info.textContent="Select an item";
    pill.textContent="No Selection";
    pill.className="tree-count-pill";
    if(icon)icon.innerHTML="";
    badges.innerHTML="";
    p.textContent="—";
    if(scriptRow)scriptRow.hidden=true;
    if(bT){bT.disabled=true;bT.hidden=true}
    if(bRen)bRen.disabled=true;
    if(bSave)bSave.disabled=true;
    renderPropertiesForm(null);
    updateSmartProtectionControls();
    return;
  }
  const cls=selectedNode.class_name||(selectedNode.is_script?(selectedNode.kind==="client_script"?"LocalScript":selectedNode.kind==="module_script"?"ModuleScript":"Script"):"Folder");
  info.textContent=selectedNode.name;
  pill.textContent=cls;
  pill.className="tree-count-pill cyan";
  if(icon)icon.innerHTML=getRobloxIcon(selectedNode.class_name||selectedNode.kind,selectedNode.name);
  p.textContent=selectedNode.path||"—";
  let badgeHtml="";
  if(selectedNode.is_script){
    if(selectedNode.enabled===false)badgeHtml+='<span class="tree-badge badge-dis">Disabled</span>';
    badgeHtml+='<span class="tree-badge '+(selectedNode.protected?"badge-protected":"badge-orig")+'">'+(selectedNode.protected?"VM":"Original")+'</span>';
  }
  badges.innerHTML=badgeHtml;
  if(scriptRow){
    scriptRow.hidden=!selectedNode.is_script;
  }
  if(bT){
    bT.hidden=!selectedNode.is_script;
    bT.disabled=!selectedNode.is_script||protecting;
    bT.textContent=(selectedNode.enabled!==false)?"Disable Script":"Enable Script";
  }
  const isRoot=isRootService(selectedNode);
  if(bRen)bRen.disabled=isRoot;
  if(bSave)bSave.disabled=false;
  renderPropertiesForm(selectedNode);
  updateSmartProtectionControls();
}
function initEnabledServices(){
  if(!explorerTree||!explorerTree.children)return;
  if(enabledServices===null){
    enabledServices=new Set();
    explorerTree.children.forEach(c=>{
      const s=c.name.toLowerCase();
      if(s!=="workspace"&&s!=="soundservice")enabledServices.add(s);
    });
  }
}
function renderServiceFilterDropdown(){
  const list=$("serviceFilterList"),badge=$("serviceFilterBadge");
  if(!list||!badge)return;
  initEnabledServices();
  const services=[];
  if(explorerTree&&explorerTree.children){
    explorerTree.children.forEach(c=>{
      if(c.kind==="service"||c.path.split("/").length===1){
        services.push(c.name);
      }
    });
  }
  if(!services.length){
    badge.textContent="0/0";
    list.innerHTML='<div style="padding:6px;color:var(--muted)">No services</div>';
    return;
  }
  const total=services.length;
  const activeCount=services.filter(s=>enabledServices&&enabledServices.has(s.toLowerCase())).length;
  badge.textContent=activeCount+"/"+total;
  let html="";
  services.forEach(s=>{
    const isChecked=enabledServices?enabledServices.has(s.toLowerCase()):true;
    html+='<label class="filter-check-item">'+
      '<input type="checkbox" data-service="'+escapeHtml(s)+'" '+(isChecked?"checked":"")+'>'+
      '<span>'+escapeHtml(s)+'</span>'+
    '</label>';
  });
  list.innerHTML=html;
  list.querySelectorAll("input[data-service]").forEach(chk=>{
    chk.onchange=()=>{
      if(!enabledServices)initEnabledServices();
      const name=chk.dataset.service.toLowerCase();
      if(chk.checked)enabledServices.add(name);
      else enabledServices.delete(name);
      renderServiceFilterDropdown();
      renderExplorer();
    };
  });
}
async function loadExplorerTree(){
  if(!selectedId)return;
  const wrap=$("explorerTreeWrap");
  try{
    const body=await api("/app/explorer/tree");
    const summary=body.summary||{};
    explorerTree=summary.tree||null;
    $("protTotalBadge").textContent=(summary.total_instances||summary.total_scripts||0)+" Items";
    $("protProtectedBadge").textContent=(summary.protected_scripts||0)+" Protected";
    $("protOrigBadge").textContent=(summary.original_scripts||0)+" Original";
    if($("protDisabledBadge"))$("protDisabledBadge").textContent=(summary.disabled_scripts||0)+" Disabled";
    if(expandedPaths.size===0&&explorerTree&&explorerTree.children){
      explorerTree.children.forEach(c=>expandedPaths.add(c.path));
    }
    renderServiceFilterDropdown();
    renderExplorer();
    if(selectedNode){selectedNode=findNodeByPath(explorerTree,selectedNode.path);updateSelectedNodeDisplay()}
    else{updateSmartProtectionControls()}
  }catch(err){wrap.innerHTML='<div class="empty">'+escapeHtml(err.message)+'</div>'}
}
const loadProtectionTree=loadExplorerTree;
function isNodeExpanded(node,depth,query){
  if(query)return true;
  return expandedPaths.has(node.path);
}
function nodeMatchesFilter(node,query){
  if(!query)return true;
  if(node.name.toLowerCase().includes(query))return true;
  if(node.children)return node.children.some(c=>nodeMatchesFilter(c,query));
  return false;
}
function renderNodeHTML(node,depth,query){
  if(depth===0&&enabledServices&&!enabledServices.has(node.name.toLowerCase())){
    return "";
  }
  if(!nodeMatchesFilter(node,query))return "";
  const hasChildren=node.children&&node.children.length>0;
  const isExpanded=isNodeExpanded(node,depth,query);
  const isChecked=checkedPaths.has(node.path);
  const isSelected=selectedNode&&selectedNode.path===node.path;
  const isRoot=depth===0||isRootService(node);
  const indent=isRoot?0:depth*14;
  let html='<div class="tree-row '+(isSelected?"selected ":"")+(isRoot?"root-service ":"")+'" data-path="'+escapeHtml(node.path)+'" style="padding-left:'+indent+'px">';
  if(hasChildren)html+='<span class="tree-caret '+(isExpanded?"":"collapsed")+'" data-toggle="'+escapeHtml(node.path)+'">▼</span>';
  else html+='<span class="tree-caret empty"></span>';
  html+='<input type="checkbox" class="tree-check" data-check="'+escapeHtml(node.path)+'" '+(isChecked?"checked":"")+'>';
  html+='<span class="tree-icon">'+getRobloxIcon(node.class_name||node.kind,node.name)+'</span>';
  html+='<span class="tree-name" title="'+escapeHtml(node.path)+'">'+escapeHtml(node.name)+'</span>';
  if(node.is_script){
    if(node.enabled===false)html+='<span class="tree-badge badge-dis" style="margin-right:4px">Dis</span>';
    html+='<span class="tree-badge '+(node.protected?"badge-protected":"badge-orig")+'">'+(node.protected?"VM":"Orig")+'</span>';
  }
  html+='</div>';
  if(hasChildren&&isExpanded){for(const child of node.children)html+=renderNodeHTML(child,depth+1,query)}
  return html;
}
function syncTreeCheckboxes(){
  $("explorerTreeWrap").querySelectorAll(".tree-check").forEach(chk=>{
    chk.checked=checkedPaths.has(chk.dataset.check);
  });
  updateSmartProtectionControls();
}
function renderExplorer(){
  const wrap=$("explorerTreeWrap");
  if(!explorerTree||!explorerTree.children||!explorerTree.children.length){wrap.innerHTML='<div class="empty">No items found in project.</div>';return}
  const query=explorerFilter.trim().toLowerCase();
  let html="";
  for(const child of explorerTree.children)html+=renderNodeHTML(child,0,query);
  wrap.innerHTML=html||'<div class="empty">No items match '+(query?'"'+escapeHtml(query)+'"':"filter")+'.</div>';
}
function formatTime(){
  const d=new Date();
  return [d.getHours(),d.getMinutes(),d.getSeconds()].map(n=>String(n).padStart(2,'0')).join(':');
}
function appendConsole(type,title,details=[]){
  const con=$("securityConsole");
  if(!con)return;
  const time=formatTime();
  const line=document.createElement("div");
  line.className="console-line "+type;
  let html='<span style="color:#64748b">['+time+']</span> <span class="console-tag '+type+'">'+escapeHtml(type.toUpperCase())+'</span> '+escapeHtml(title);
  if(details&&details.length){
    for(const d of details){
      html+='\n  <span style="color:#64748b">↳</span> '+escapeHtml(d);
    }
  }
  line.innerHTML=html;
  con.appendChild(line);
  con.scrollTop=con.scrollHeight;
}
function clearConsole(){
  const con=$("securityConsole");
  if(con){
    con.innerHTML='<div class="console-line"><span style="color:#64748b">['+formatTime()+']</span> <span class="console-tag info">INFO</span> Console cleared. Ready.</div>';
  }
}
function getSmartTargetInfo(){
  const checkedScripts=getCheckedScriptPaths();
  if(checkedScripts&&checkedScripts.length>0){
    return {
      mode:"batch",
      count:checkedScripts.length,
      paths:checkedScripts,
      name:checkedScripts.length+" Scripts",
      label:"Batch ("+checkedScripts.length+")",
      protected:false,
      canProtect:true,
      canRestore:true
    };
  }
  if(selectedNode){
    if(selectedNode.is_script){
      const sPath=selectedNode.script_rel_path||selectedNode.path;
      return {
        mode:"single",
        count:1,
        paths:[sPath],
        name:selectedNode.name,
        label:selectedNode.name,
        protected:!!selectedNode.protected,
        canProtect:!selectedNode.protected,
        canRestore:!!selectedNode.protected
      };
    }
    const folderScripts=collectNodeScripts(selectedNode);
    if(folderScripts&&folderScripts.length>0){
      return {
        mode:"folder",
        count:folderScripts.length,
        paths:folderScripts,
        name:selectedNode.name,
        label:selectedNode.name+" ("+folderScripts.length+")",
        protected:false,
        canProtect:true,
        canRestore:true
      };
    }
  }
  return {
    mode:"none",
    count:0,
    paths:[],
    name:"",
    label:"No Selection",
    protected:false,
    canProtect:false,
    canRestore:false
  };
}
function updateSmartProtectionControls(){
  const info=getSmartTargetInfo();
  const badge=$("smartTargetBadge"),desc=$("smartTargetDesc"),btnP=$("smartProtectBtn"),btnR=$("smartRestoreBtn");
  if(badge){
    badge.textContent=info.label;
    badge.className="tree-count-pill "+(info.mode==="batch"?"amber":info.mode!=="none"?"cyan":"");
  }
  if(desc){
    if(info.mode==="batch"){
      desc.textContent="Batch: "+info.count+" script(s) checked in Explorer";
    }else if(info.mode==="single"){
      desc.textContent="Single: "+info.name+(info.protected?" [VM Protected]":" [Original]");
    }else if(info.mode==="folder"){
      desc.textContent="Folder: "+info.name+" ("+info.count+" scripts inside)";
    }else{
      desc.textContent="Select a script or check items in Explorer";
    }
  }
  if(btnP){
    if(info.mode==="batch"){
      btnP.textContent="Protect "+info.count+" Scripts";
      btnP.disabled=protecting||info.count===0;
    }else if(info.mode==="single"){
      btnP.textContent="Protect Script (VM)";
      btnP.disabled=protecting||!info.canProtect;
    }else if(info.mode==="folder"){
      btnP.textContent="Protect Folder ("+info.count+")";
      btnP.disabled=protecting||info.count===0;
    }else{
      btnP.textContent="Protect VM";
      btnP.disabled=true;
    }
  }
  if(btnR){
    if(info.mode==="batch"){
      btnR.textContent="Restore "+info.count+" Scripts";
      btnR.disabled=protecting||info.count===0;
    }else if(info.mode==="single"){
      btnR.textContent="Restore Original";
      btnR.disabled=protecting||!info.canRestore;
    }else if(info.mode==="folder"){
      btnR.textContent="Restore Folder ("+info.count+")";
      btnR.disabled=protecting||info.count===0;
    }else{
      btnR.textContent="Restore Original";
      btnR.disabled=true;
    }
  }
}
async function doProtect(paths){
  if(!paths||!paths.length){toast("No scripts selected to protect.","warning");return}
  protecting=true;
  updateSmartProtectionControls();
  appendConsole("info","Protecting "+paths.length+" script(s) with Sorevium VM...");
  try{
    const watermark=$("protWatermarkInput")?$("protWatermarkInput").value:"";
    const body=await api("/app/explorer/protect",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({paths,watermark})});
    const processed=body.processed||[];
    const count=processed.length;
    appendConsole("protect","Successfully protected "+count+" script(s) with Sorevium VM:",processed);
    toast(count+" script(s) protected with Sorevium VM.","success");
    await loadExplorerTree();
  }catch(err){
    appendConsole("error","Protection failed: "+err.message);
    toast(err.message,"error");
  }finally{
    protecting=false;
    updateSelectedNodeDisplay();
    updateSmartProtectionControls();
  }
}
async function doRestore(paths){
  if(!paths||!paths.length){toast("No scripts selected to restore.","warning");return}
  protecting=true;
  updateSmartProtectionControls();
  appendConsole("info","Restoring "+paths.length+" script(s) to original Luau...");
  try{
    const body=await api("/app/explorer/restore",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({paths})});
    const restored=body.restored||[];
    const count=restored.length;
    appendConsole("restore","Successfully restored "+count+" script(s) from vault to original:",restored);
    toast(count+" script(s) restored to original.","success");
    await loadExplorerTree();
  }catch(err){
    appendConsole("error","Restore failed: "+err.message);
    toast(err.message,"error");
  }finally{
    protecting=false;
    updateSelectedNodeDisplay();
    updateSmartProtectionControls();
  }
}
async function doSetEnabled(paths,enabled){
  if(!paths||!paths.length){toast("No scripts selected.","warning");return}
  protecting=true;
  updateSmartProtectionControls();
  appendConsole("info",(enabled?"Enabling ":"Disabling ")+paths.length+" script(s)...");
  try{
    const body=await api("/app/explorer/set-enabled",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({paths,enabled})});
    const list=body.updated||body.processed||[];
    const count=list.length;
    appendConsole("state","Successfully "+(enabled?"enabled ":"disabled ")+count+" script(s):",list);
    toast(count+" script(s) "+(enabled?"enabled.":"disabled."),"success");
    await loadExplorerTree();
  }catch(err){
    appendConsole("error","Failed to set state: "+err.message);
    toast(err.message,"error");
  }finally{
    protecting=false;
    updateSelectedNodeDisplay();
    updateSmartProtectionControls();
  }
}
async function doReveal(path){
  if(!path)return;
  try{
    await api("/app/explorer/reveal",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({path})});
  }catch(err){toast(err.message,"error")}
}
function requestRename(node){
  if(!node||isRootService(node))return;
  $("renameInput").value=node.name;
  $("renameSub").textContent="Path: "+(node.path||"");
  openModal("renameModal");
  $("renameInput").focus();
  $("renameInput").select();
}
async function confirmRename(){
  if(!selectedNode)return;
  const newName=$("renameInput").value.trim();
  if(!newName||newName===selectedNode.name){closeModal("renameModal");return}
  closeModal("renameModal");
  try{
    const res=await api("/app/explorer/rename",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({path:selectedNode.path,new_name:newName})});
    toast("Renamed to "+newName,"success");
    selectedNode.path=res.new_path||selectedNode.path;
    selectedNode.name=newName;
    await loadExplorerTree();
  }catch(err){
    toast("Rename failed: "+err.message,"error");
  }
}
function requestDelete(node){
  if(!node||isRootService(node))return;
  $("deleteNodeMsg").textContent='Are you sure you want to delete "'+node.name+'" from disk and Roblox Studio?';
  openModal("deleteNodeModal");
}
async function confirmDelete(){
  if(!selectedNode)return;
  closeModal("deleteNodeModal");
  try{
    await api("/app/explorer/delete",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({path:selectedNode.path})});
    toast("Deleted "+selectedNode.name,"success");
    selectedNode=null;
    updateSelectedNodeDisplay();
    await loadExplorerTree();
  }catch(err){
    toast("Delete failed: "+err.message,"error");
  }
}
function showContextMenu(e,node){
  e.preventDefault();
  if(!node)return;
  selectedNode=node;
  updateSelectedNodeDisplay();
  const menu=$("explorerContextMenu");
  if(!menu)return;
  const isService=isRootService(node);
  let html="";
  if(!isService){
    html+='<div class="context-menu-item" id="ctxRename"><span>Rename</span><small style="color:var(--muted)">F2</small></div>';
    html+='<div class="context-menu-item danger" id="ctxDelete"><span>Delete</span><small style="color:var(--muted)">Del</small></div>';
    html+='<div class="context-menu-divider"></div>';
  }
  if(node.is_script){
    const enText=(node.enabled!==false)?"Disable Script":"Enable Script";
    html+='<div class="context-menu-item" id="ctxToggle">'+enText+'</div>';
    html+='<div class="context-menu-divider"></div>';
  }
  html+='<div class="context-menu-item" id="ctxReveal"><span>Reveal in Finder</span></div>';
  html+='<div class="context-menu-item" id="ctxCopy"><span>Copy Path</span></div>';
  menu.innerHTML=html;
  menu.style.display="block";
  menu.style.left=Math.min(e.clientX,window.innerWidth-160)+"px";
  menu.style.top=Math.min(e.clientY,window.innerHeight-150)+"px";
  const mRen=$("ctxRename"),mDel=$("ctxDelete"),mTog=$("ctxToggle"),mRev=$("ctxReveal"),mCopy=$("ctxCopy");
  if(mRen)mRen.onclick=()=>{hideContextMenu();requestRename(node)};
  if(mDel)mDel.onclick=()=>{hideContextMenu();requestDelete(node)};
  if(mTog)mTog.onclick=()=>{hideContextMenu();doSetEnabled([(node.script_rel_path||node.path)],node.enabled===false)};
  if(mRev)mRev.onclick=()=>{hideContextMenu();doReveal(node.path)};
  if(mCopy)mCopy.onclick=()=>{hideContextMenu();copyText(node.path).then(ok=>toast(ok?"Path copied.":"Copy failed.",ok?"success":"error"))};
}
function hideContextMenu(){
  const menu=$("explorerContextMenu");
  if(menu)menu.style.display="none";
}
document.addEventListener("click",()=>hideContextMenu());
$("explorerTreeWrap").oncontextmenu=e=>{
  const row=e.target.closest(".tree-row");
  if(row){
    const path=row.dataset.path;
    const node=findNodeByPath(explorerTree,path);
    if(node)showContextMenu(e,node);
  }
};
$("explorerTreeWrap").onclick=e=>{
  hideContextMenu();
  const caret=e.target.closest(".tree-caret:not(.empty)");
  if(caret){
    e.stopPropagation();
    const path=caret.dataset.toggle;
    if(expandedPaths.has(path))expandedPaths.delete(path);
    else expandedPaths.add(path);
    renderExplorer();
    return;
  }
  const chk=e.target.closest(".tree-check");
  if(chk){
    e.stopPropagation();
    const path=chk.dataset.check;
    const node=findNodeByPath(explorerTree,path);
    if(!node)return;
    const allPaths=collectAllNodePaths(node);
    if(chk.checked){
      for(const p of allPaths)checkedPaths.add(p);
    }else{
      for(const p of allPaths)checkedPaths.delete(p);
    }
    syncTreeCheckboxes();
    return;
  }
  const row=e.target.closest(".tree-row");
  if(row){
    const path=row.dataset.path;
    selectedNode=findNodeByPath(explorerTree,path);
    $("explorerTreeWrap").querySelectorAll(".tree-row.selected").forEach(r=>r.classList.remove("selected"));
    row.classList.add("selected");
    updateSelectedNodeDisplay();
  }
};
$("explorerTreeWrap").ondblclick=e=>{
  const row=e.target.closest(".tree-row");
  if(!row)return;
  const path=row.dataset.path;
  const node=findNodeByPath(explorerTree,path);
  if(node&&node.children&&node.children.length>0){
    if(expandedPaths.has(path))expandedPaths.delete(path);
    else expandedPaths.add(path);
    renderExplorer();
  }
};
function getCheckedScriptPaths(){
  const result=new Set();
  for(const p of checkedPaths){
    const node=findNodeByPath(explorerTree,p);
    if(node){
      if(node.is_script)result.add(node.script_rel_path||node.path);
      else{for(const s of collectNodeScripts(node))result.add(s)}
    }
  }
  return [...result];
}
$("startBtn").onclick=()=>action("/app/start");$("stopBtn").onclick=()=>action("/app/stop");$("startAllBtn").onclick=startAll;$("openBtn").onclick=()=>action("/app/open-folder");$("configOpenBtn").onclick=()=>action("/app/open-folder");$("changePathBtn").onclick=()=>setPanel("configPanel");
$("smartProtectBtn").onclick=()=>{const t=getSmartTargetInfo();if(t.paths&&t.paths.length)doProtect(t.paths)};
$("smartRestoreBtn").onclick=()=>{const t=getSmartTargetInfo();if(t.paths&&t.paths.length)doRestore(t.paths)};
$("clearConsoleBtn").onclick=()=>clearConsole();
$("toggleSingleEnabledBtn").onclick=()=>{if(!selectedNode||!selectedNode.is_script)return;doSetEnabled([selectedNode.script_rel_path||selectedNode.path],selectedNode.enabled===false)};
$("renameNodeBtn").onclick=()=>{if(selectedNode&&!isRootService(selectedNode))requestRename(selectedNode)};
$("confirmRenameBtn").onclick=()=>confirmRename();$("cancelRenameBtn").onclick=()=>closeModal("renameModal");$("confirmDeleteNodeBtn").onclick=()=>confirmDelete();$("cancelDeleteNodeBtn").onclick=()=>closeModal("deleteNodeModal");
$("explorerRefreshBtn").onclick=()=>loadExplorerTree();
$("serviceFilterDropdownBtn").onclick=e=>{e.stopPropagation();const m=$("serviceFilterDropdownMenu");if(m)m.hidden=!m.hidden};
$("filterSelectAllBtn").onclick=()=>{if(!explorerTree||!explorerTree.children)return;if(!enabledServices)enabledServices=new Set();explorerTree.children.forEach(c=>enabledServices.add(c.name.toLowerCase()));renderServiceFilterDropdown();renderExplorer()};
$("filterDeselectAllBtn").onclick=()=>{if(!enabledServices)enabledServices=new Set();else enabledServices.clear();renderServiceFilterDropdown();renderExplorer()};
document.addEventListener("click",e=>{const m=$("serviceFilterDropdownMenu");if(m&&!m.hidden&&!e.target.closest(".filter-dropdown-wrap"))m.hidden=true});
$("explorerSearchInput").oninput=e=>{clearTimeout(searchTimer);searchTimer=setTimeout(()=>{explorerFilter=e.target.value;renderExplorer()},200)};
$("subTabProps").onclick=()=>setSubTab("props");$("subTabSecurity").onclick=()=>setSubTab("security");$("savePropsBtn").onclick=()=>saveProperties();
$("historyRefreshBtn").onclick=loadHistory;$("execHistoryRefreshBtn").onclick=loadExecHistory;$("execRunBtn").onclick=runExec;$("execLatestBtn").onclick=()=>rerunExec("");$("execTokenCopyBtn").onclick=()=>copyText($("execTokenDisplay").value).then(ok=>toast(ok?"Token copied. Paste it into the Studio profile.":"Copy failed. Select the token and copy it manually.",ok?"success":"error"));
$("applyConfigBtn").onclick=saveConfig;$("renameBtn").onclick=renameProject;$("removeProjectBtn").onclick=e=>requestRemoveProject(e.currentTarget);$("syncRootPickerBtn").onclick=pickConfigFolder;
$("diagnosticsToggle").onclick=()=>setDisclosure("diagnosticsToggle","diagnosticsBody",$("diagnosticsToggle").getAttribute("aria-expanded")!=="true");$("advancedToggle").onclick=()=>setDisclosure("advancedToggle","advancedBody",$("advancedToggle").getAttribute("aria-expanded")!=="true");
$("addProjectBtn").onclick=e=>openModal("addModal",e.currentTarget);$("addCloseBtn").onclick=()=>closeModal("addModal");$("addPickBtn").onclick=pickNewFolder;$("addCreateBtn").onclick=addProject;$("removeCancelBtn").onclick=closeRemoveModal;$("removeKeepBtn").onclick=()=>confirmRemoveProject(false);$("removeDeleteBtn").onclick=()=>confirmRemoveProject(true);
$("viewCloseBtn").onclick=()=>closeModal("viewModal");$("viewCopyBtn").onclick=()=>copyText(selectedEntry?.source||"").then(ok=>toast(ok?"Source copied.":"Copy failed. Select the source and copy it manually.",ok?"success":"error"));
$("errorDetailsBtn").onclick=()=>{$("errorDetailText").textContent=$("errorDetailsBtn").dataset.report||"";openModal("errorModal",$("errorDetailsBtn"))};$("closeErrorBtn").onclick=()=>closeModal("errorModal");$("copyErrorBtn").onclick=()=>copyText($("errorDetailText").textContent).then(ok=>toast(ok?"Report copied.":"Copy failed. Select and copy the report manually.",ok?"success":"error"));
$("logsBtn").onclick=e=>{openModal("logsModal",e.currentTarget);loadLogs()};$("closeLogsBtn").onclick=()=>closeModal("logsModal");$("refreshLogsBtn").onclick=loadLogs;
["viewModal","addModal","logsModal","removeModal","errorModal","renameModal","deleteNodeModal"].forEach(id=>$(id)&&($(id).onclick=e=>{if(e.target.id!==id)return;if(id==="removeModal"){if(!$("removeKeepBtn").disabled)closeRemoveModal();return}closeModal(id)}));
document.addEventListener("keydown",e=>{
  const modal=document.querySelector(".modal-backdrop.open");
  if(modal){
    if(e.key==="Escape"){
      e.preventDefault();
      if(modal.id==="removeModal"){if(!$("removeKeepBtn").disabled)closeRemoveModal()}
      else closeModal(modal.id);
      return;
    }
    if(e.key==="Enter"&&modal.id==="renameModal"){
      e.preventDefault();
      confirmRename();
      return;
    }
    if(e.key!=="Tab")return;
    const focusable=[...modal.querySelectorAll("button:not(:disabled),input:not(:disabled),textarea:not(:disabled),[tabindex]:not([tabindex='-1'])")].filter(node=>node.offsetParent!==null);
    if(!focusable.length)return;
    const first=focusable[0],last=focusable[focusable.length-1];
    if(e.shiftKey&&document.activeElement===first){e.preventDefault();last.focus()}
    else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first.focus()}
    return;
  }
  if(["INPUT","TEXTAREA","SELECT"].includes(document.activeElement?.tagName))return;
  if(selectedNode&&!isRootService(selectedNode)){
    if(e.key==="F2"){
      e.preventDefault();
      requestRename(selectedNode);
    }else if(e.key==="Delete"||(e.key==="Backspace"&&(e.metaKey||e.ctrlKey))){
      e.preventDefault();
      requestDelete(selectedNode);
    }
  }
});
async function boot(){await loadInstances(true);await loadHistory();await loadExecHistory();await loadProtectionTree();setInterval(async()=>{await loadInstances(false);await refreshSelected()},1200)}
boot();
</script>
</body>
</html>`
}
