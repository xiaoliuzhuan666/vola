use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream, ToSocketAddrs};
use std::path::PathBuf;
use std::process::{Child, Command};
use std::sync::{Arc, Mutex};
use std::time::Duration;
use tauri::Manager;

// 保存 Go 守护进程句柄的全局 State 结构体
struct BackendProcess {
    child: Arc<Mutex<Option<Child>>>,
    api_base: Arc<Mutex<Option<String>>>,
}

#[derive(serde::Serialize)]
struct CliToolsInstallResult {
    source_path: String,
    install_dir: String,
    commands: Vec<String>,
    command_paths: Vec<String>,
    path_updated: bool,
    rc_file: Option<String>,
    shell_reload_command: Option<String>,
}

const BACKEND_HOST: &str = "127.0.0.1";
const BACKEND_PORT_START: u16 = 42690;
const BACKEND_PORT_END: u16 = 42719;
const DEFAULT_API_BASE: &str = "http://127.0.0.1:42690";

#[tauri::command]
fn get_api_base(state: tauri::State<'_, BackendProcess>) -> String {
    if let Ok(lock) = state.api_base.lock() {
        if let Some(api_base) = lock.as_ref() {
            return api_base.clone();
        }
    }

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
            if let Some(api_base) = read_runtime_api_base(path) {
                if api_base_reachable(&api_base) {
                    return api_base;
                }
            }
        }
        std::thread::sleep(std::time::Duration::from_millis(500));
    }

    // 默认退回
    DEFAULT_API_BASE.to_string()
}

fn read_runtime_api_base(path: &str) -> Option<String> {
    let content = std::fs::read_to_string(path).ok()?;
    let val = serde_json::from_str::<serde_json::Value>(&content).ok()?;
    val.get("api_base")
        .and_then(|v| v.as_str())
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
}

fn api_base_reachable(api_base: &str) -> bool {
    let Some(endpoint) = api_base
        .trim()
        .trim_end_matches('/')
        .strip_prefix("http://")
        .map(|value| value.split('/').next().unwrap_or(""))
    else {
        return false;
    };
    if endpoint.is_empty() {
        return false;
    }
    let Ok(addresses) = endpoint.to_socket_addrs() else {
        return false;
    };
    addresses.into_iter().any(|addr| {
        let Ok(mut stream) = TcpStream::connect_timeout(&addr, Duration::from_millis(250)) else {
            return false;
        };
        let _ = stream.set_read_timeout(Some(Duration::from_millis(500)));
        let _ = stream.set_write_timeout(Some(Duration::from_millis(500)));
        let request =
            format!("GET /api/config HTTP/1.1\r\nHost: {endpoint}\r\nConnection: close\r\n\r\n");
        if stream.write_all(request.as_bytes()).is_err() {
            return false;
        }
        let mut buf = [0_u8; 64];
        match stream.read(&mut buf) {
            Ok(n) if n > 0 => {
                let response = String::from_utf8_lossy(&buf[..n]);
                response.starts_with("HTTP/1.1 200") || response.starts_with("HTTP/1.0 200")
            }
            _ => false,
        }
    })
}

fn port_available(port: u16) -> bool {
    TcpListener::bind((BACKEND_HOST, port)).is_ok()
}

fn choose_backend_port() -> u16 {
    for port in BACKEND_PORT_START..=BACKEND_PORT_END {
        if port_available(port) {
            return port;
        }
    }
    TcpListener::bind((BACKEND_HOST, 0))
        .and_then(|listener| listener.local_addr())
        .map(|addr| addr.port())
        .unwrap_or(BACKEND_PORT_START)
}

fn home_dir() -> Result<PathBuf, String> {
    if let Ok(home) = std::env::var("HOME") {
        if !home.trim().is_empty() {
            return Ok(PathBuf::from(home));
        }
    }
    if let Ok(profile) = std::env::var("USERPROFILE") {
        if !profile.trim().is_empty() {
            return Ok(PathBuf::from(profile));
        }
    }
    Err("无法确定当前用户的 home 目录".to_string())
}

fn resolve_bundled_vola_binary(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    let executable_name = if cfg!(windows) { "vola.exe" } else { "vola" };
    let mut candidates = Vec::new();

    if let Ok(res_dir) = app.path().resource_dir() {
        candidates.push(res_dir.join("bin").join(executable_name));
        candidates.push(res_dir.join("bin/vola"));
    }

    if let Ok(exe_path) = std::env::current_exe() {
        if let Some(parent) = exe_path.parent() {
            candidates.push(parent.join(executable_name));
            candidates.push(parent.join("vola"));
            candidates.push(parent.join("bin").join(executable_name));
            candidates.push(parent.join("bin/vola"));
        }
    }

    #[cfg(debug_assertions)]
    {
        candidates.push(PathBuf::from("./bin").join(executable_name));
        candidates.push(PathBuf::from("./bin/vola"));
    }

    candidates
        .into_iter()
        .find(|path| path.is_file())
        .ok_or_else(|| "未找到桌面版内置的 Vola 命令，请重新安装或更新 Vola。".to_string())
}

fn cli_install_dir() -> Result<PathBuf, String> {
    if cfg!(windows) {
        if let Ok(local_app_data) = std::env::var("LOCALAPPDATA") {
            if !local_app_data.trim().is_empty() {
                return Ok(PathBuf::from(local_app_data).join("Vola").join("bin"));
            }
        }
    }
    Ok(home_dir()?.join(".local").join("bin"))
}

fn command_names() -> Vec<String> {
    let suffix = if cfg!(windows) { ".exe" } else { "" };
    ["neu", "vola", "vol", "neudrive", "xlzdrive"]
        .iter()
        .map(|name| format!("{name}{suffix}"))
        .collect()
}

fn path_contains_dir(dir: &std::path::Path) -> bool {
    let Some(path_var) = std::env::var_os("PATH") else {
        return false;
    };
    std::env::split_paths(&path_var).any(|entry| entry == dir)
}

fn preferred_shell_rc(home: &std::path::Path) -> Option<PathBuf> {
    let shell = std::env::var("SHELL").unwrap_or_default();
    let shell_name = PathBuf::from(shell)
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("")
        .to_string();
    match shell_name.as_str() {
        "zsh" => Some(home.join(".zshrc")),
        "bash" => {
            let bashrc = home.join(".bashrc");
            if bashrc.exists() {
                Some(bashrc)
            } else {
                Some(home.join(".bash_profile"))
            }
        }
        "fish" => Some(home.join(".config").join("fish").join("config.fish")),
        _ => None,
    }
}

fn ensure_install_dir_on_shell_path(
    install_dir: &std::path::Path,
) -> Result<(bool, Option<PathBuf>, Option<String>), String> {
    if cfg!(windows) || path_contains_dir(install_dir) {
        return Ok((false, None, None));
    }

    let home = home_dir()?;
    let Some(rc_file) = preferred_shell_rc(&home) else {
        return Ok((false, None, None));
    };
    let shell_name = std::env::var("SHELL")
        .ok()
        .and_then(|shell| {
            PathBuf::from(shell)
                .file_name()
                .map(|name| name.to_string_lossy().into_owned())
        })
        .unwrap_or_default();
    let line = if shell_name == "fish" {
        format!("fish_add_path -m {}", install_dir.display())
    } else if install_dir == home.join(".local").join("bin") {
        "export PATH=\"$HOME/.local/bin:$PATH\"".to_string()
    } else {
        format!("export PATH=\"{}:$PATH\"", install_dir.display())
    };

    if let Some(parent) = rc_file.parent() {
        std::fs::create_dir_all(parent).map_err(|err| format!("创建 shell 配置目录失败: {err}"))?;
    }
    let current = std::fs::read_to_string(&rc_file).unwrap_or_default();
    if !current.contains(&line) {
        use std::io::Write;
        let mut file = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(&rc_file)
            .map_err(|err| format!("打开 shell 配置失败: {err}"))?;
        writeln!(file, "\n# Added by Vola installer\n{line}")
            .map_err(|err| format!("写入 shell 配置失败: {err}"))?;
    }

    let reload = Some(format!("source {}", rc_file.display()));
    Ok((true, Some(rc_file), reload))
}

#[cfg(unix)]
fn make_executable(path: &std::path::Path) -> Result<(), String> {
    use std::os::unix::fs::PermissionsExt;
    let mut perms = std::fs::metadata(path)
        .map_err(|err| format!("读取命令权限失败: {err}"))?
        .permissions();
    perms.set_mode(0o755);
    std::fs::set_permissions(path, perms).map_err(|err| format!("设置命令权限失败: {err}"))
}

#[cfg(not(unix))]
fn make_executable(_path: &std::path::Path) -> Result<(), String> {
    Ok(())
}

#[tauri::command]
fn install_cli_tools(app: tauri::AppHandle) -> Result<CliToolsInstallResult, String> {
    let source = resolve_bundled_vola_binary(&app)?;
    let install_dir = cli_install_dir()?;
    std::fs::create_dir_all(&install_dir).map_err(|err| format!("创建命令目录失败: {err}"))?;
    let bytes = std::fs::read(&source).map_err(|err| format!("读取内置命令失败: {err}"))?;

    let mut command_paths = Vec::new();
    let commands = command_names();
    for command in &commands {
        let target = install_dir.join(command);
        std::fs::write(&target, &bytes).map_err(|err| format!("安装 {command} 失败: {err}"))?;
        make_executable(&target)?;
        command_paths.push(target.to_string_lossy().into_owned());
    }

    let (path_updated, rc_file, shell_reload_command) =
        ensure_install_dir_on_shell_path(&install_dir)?;

    Ok(CliToolsInstallResult {
        source_path: source.to_string_lossy().into_owned(),
        install_dir: install_dir.to_string_lossy().into_owned(),
        commands,
        command_paths,
        path_updated,
        rc_file: rc_file.map(|path| path.to_string_lossy().into_owned()),
        shell_reload_command,
    })
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let child_state = Arc::new(Mutex::new(None));
    let child_state_clone = child_state.clone();
    let api_base_state = Arc::new(Mutex::new(None));
    let api_base_state_clone = api_base_state.clone();

    tauri::Builder::default()
        .plugin(tauri_plugin_log::Builder::default().build())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .manage(BackendProcess {
            child: child_state,
            api_base: api_base_state,
        })
        .setup(move |app| {
            match resolve_bundled_vola_binary(app.handle()) {
                Ok(bin_path) => {
                    println!("Launching Go backend from: {:?}", bin_path);
                    let port = choose_backend_port();
                    let listen_addr = format!("{BACKEND_HOST}:{port}");
                    let api_base = format!("http://{listen_addr}");
                    let child = Command::new(bin_path)
                        .arg("server")
                        .arg("--local-mode")
                        .arg("--listen")
                        .arg(&listen_addr)
                        .arg("--storage")
                        .arg("sqlite")
                        .arg("--public-base-url")
                        .arg(&api_base)
                        .spawn();

                    match child {
                        Ok(c) => {
                            if let Ok(mut lock) = api_base_state_clone.lock() {
                                *lock = Some(api_base);
                            }
                            *child_state_clone.lock().unwrap() = Some(c);
                            println!("Go backend started successfully.");
                        }
                        Err(e) => {
                            eprintln!("Failed to spawn Go backend: {:?}", e);
                        }
                    }
                }
                Err(e) => {
                    eprintln!("{e}");
                }
            }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![get_api_base, install_cli_tools])
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
