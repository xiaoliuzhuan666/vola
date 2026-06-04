use std::sync::{Arc, Mutex};
use std::path::PathBuf;
use std::process::{Command, Child};
use tauri::Manager;

// 保存 Go 守护进程句柄的全局 State 结构体
struct BackendProcess {
    child: Arc<Mutex<Option<Child>>>,
}

#[tauri::command]
fn get_api_base() -> String {
    let home = std::env::var("HOME").unwrap_or_default();

    // 兼容传统路径和新路径
    let paths = vec![
        format!("{}/.config/vola/runtime.json", home),
        format!("{}/Library/Application Support/Vola/runtime.json", home),
        format!("{}/Library/Application Support/NeuDrive/runtime.json", home),
    ];

    // 轮询最多 10 次（共 5 秒），等待 Go 进程把 runtime.json 写完
    for _ in 0..10 {
        for path in &paths {
            if let Ok(content) = std::fs::read_to_string(path) {
                if let Ok(val) = serde_json::from_str::<serde_json::Value>(&content) {
                    if let Some(api_base) = val.get("api_base").and_then(|v| v.as_str()) {
                        return api_base.to_string();
                    }
                }
            }
        }
        std::thread::sleep(std::time::Duration::from_millis(500));
    }

    // 默认退回
    "http://127.0.0.1:42690".to_string()
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let child_state = Arc::new(Mutex::new(None));
    let child_state_clone = child_state.clone();

    tauri::Builder::default()
        .plugin(tauri_plugin_log::Builder::default().build())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .manage(BackendProcess {
            child: child_state,
        })
        .setup(move |app| {
            // 查找 vola 后端二进制文件的位置
            let mut bin_path = PathBuf::from("./bin/vola");

            // 1. 如果在当前开发路径，看看二进制是否存在
            if !bin_path.exists() {
                // 2. 如果是打包好的环境，从资源目录读取
                if let Ok(res_dir) = app.path().resource_dir() {
                    bin_path = res_dir.join("bin/vola");
                }
            }

            // 3. 如果还是没有，尝试从可执行文件所在目录找
            if !bin_path.exists() {
                if let Ok(exe_path) = std::env::current_exe() {
                    if let Some(parent) = exe_path.parent() {
                        bin_path = parent.join("vola");
                        if !bin_path.exists() {
                            bin_path = parent.join("bin/vola");
                        }
                    }
                }
            }

            if bin_path.exists() {
                println!("Launching Go backend from: {:?}", bin_path);
                let child = Command::new(bin_path)
                    .arg("server")
                    .arg("--local-mode")
                    .arg("--storage")
                    .arg("sqlite")
                    .spawn();

                match child {
                    Ok(c) => {
                        *child_state_clone.lock().unwrap() = Some(c);
                        println!("Go backend started successfully.");
                    }
                    Err(e) => {
                        eprintln!("Failed to spawn Go backend: {:?}", e);
                    }
                }
            } else {
                eprintln!("Go backend binary 'vola' not found. Please compile it first.");
            }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![get_api_base])
        .build(tauri::generate_context!())
        .expect("error while building tauri application")
        .run(move |handle, event| match event {
            tauri::RunEvent::Exit => {
                let state = handle.state::<BackendProcess>();
                let mut lock = state.child.lock().unwrap();
                if let Some(mut child) = lock.take() {
                    match child.kill() {
                        Ok(_) => println!("Successfully killed Go backend on exit."),
                        Err(e) => eprintln!("Failed to kill Go backend: {:?}", e),
                    }
                }
            }
            _ => {}
        });
}
