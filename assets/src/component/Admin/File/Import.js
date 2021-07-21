import { Dialog } from "@material-ui/core";
import Button from "@material-ui/core/Button";
import Chip from "@material-ui/core/Chip";
import DialogActions from "@material-ui/core/DialogActions";
import DialogTitle from "@material-ui/core/DialogTitle";
import Fade from "@material-ui/core/Fade";
import FormControl from "@material-ui/core/FormControl";
import FormControlLabel from "@material-ui/core/FormControlLabel";
import FormHelperText from "@material-ui/core/FormHelperText";
import Input from "@material-ui/core/Input";
import InputAdornment from "@material-ui/core/InputAdornment";
import InputLabel from "@material-ui/core/InputLabel";
import MenuItem from "@material-ui/core/MenuItem";
import Paper from "@material-ui/core/Paper";
import Popper from "@material-ui/core/Popper";
import Select from "@material-ui/core/Select";
import { makeStyles } from "@material-ui/core/styles";
import Switch from "@material-ui/core/Switch";
import Typography from "@material-ui/core/Typography";
import Alert from "@material-ui/lab/Alert";
import React, { useCallback, useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { useHistory } from "react-router";
import { toggleSnackbar } from "../../../actions";
import API from "../../../middleware/Api";
import PathSelector from "../../FileManager/PathSelector";

const useStyles = makeStyles((theme) => ({
    root: {
        [theme.breakpoints.up("md")]: {
            marginLeft: 100,
        },
        marginBottom: 40,
    },
    form: {
        maxWidth: 400,
        marginTop: 20,
        marginBottom: 20,
    },
    formContainer: {
        [theme.breakpoints.up("md")]: {
            padding: "0px 24px 0 24px",
        },
    },
    userSelect: {
        width: 400,
        borderRadius: 0,
    },
}));

function useDebounce(value, delay) {
    const [debouncedValue, setDebouncedValue] = useState(value);

    useEffect(() => {
        const handler = setTimeout(() => {
            setDebouncedValue(value);
        }, delay);
        return () => {
            clearTimeout(handler);
        };
    }, [value]);

    return debouncedValue;
}

export default function Import() {
    const classes = useStyles();
    const [loading, setLoading] = useState(false);
    const [options, setOptions] = useState({
        policy: 1,
        userInput: "",
        src: "",
        dst: "",
        recursive: true,
    });
    const [anchorEl, setAnchorEl] = useState(null);
    const [policies, setPolicies] = useState({});
    const [users, setUsers] = useState([]);
    const [user, setUser] = useState(null);
    const [selectRemote, setSelectRemote] = useState(false);
    const [selectLocal, setSelectLocal] = useState(false);

    const handleChange = (name) => (event) => {
        setOptions({
            ...options,
            [name]: event.target.value,
        });
    };

    const handleCheckChange = (name) => (event) => {
        setOptions({
            ...options,
            [name]: event.target.checked,
        });
    };

    const history = useHistory();
    const dispatch = useDispatch();
    const ToggleSnackbar = useCallback(
        (vertical, horizontal, msg, color) =>
            dispatch(toggleSnackbar(vertical, horizontal, msg, color)),
        [dispatch]
    );

    const submit = (e) => {
        e.preventDefault();
        if (user === null) {
            ToggleSnackbar("top", "right", "請先選擇目標使用者", "warning");
            return;
        }
        setLoading(true);
        API.post("/admin/task/import", {
            uid: user.ID,
            policy_id: parseInt(options.policy),
            src: options.src,
            dst: options.dst,
            recursive: options.recursive,
        })
            .then(() => {
                setLoading(false);
                history.push("/admin/file");
                ToggleSnackbar(
                    "top",
                    "right",
                    "匯入任務已建立，您可以在「持久任務」中查看執行情況",
                    "success"
                );
            })
            .catch((error) => {
                setLoading(false);
                ToggleSnackbar("top", "right", error.message, "error");
            });
    };

    const debouncedSearchTerm = useDebounce(options.userInput, 500);

    useEffect(() => {
        if (debouncedSearchTerm !== "") {
            API.post("/admin/user/list", {
                page: 1,
                page_size: 10000,
                order_by: "id asc",
                searches: {
                    nick: debouncedSearchTerm,
                    email: debouncedSearchTerm,
                },
            })
                .then((response) => {
                    setUsers(response.data.items);
                })
                .catch((error) => {
                    ToggleSnackbar("top", "right", error.message, "error");
                });
        }
    }, [debouncedSearchTerm]);

    useEffect(() => {
        API.post("/admin/policy/list", {
            page: 1,
            page_size: 10000,
            order_by: "id asc",
            conditions: {},
        })
            .then((response) => {
                const res = {};
                response.data.items.forEach((v) => {
                    res[v.ID] = v;
                });
                setPolicies(res);
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            });
        // eslint-disable-next-line
    }, []);

    const selectUser = (u) => {
        setOptions({
            ...options,
            userInput: "",
        });
        setUser(u);
    };

    const setMoveTarget = (setter) => (folder) => {
        const path =
            folder.path === "/"
                ? folder.path + folder.name
                : folder.path + "/" + folder.name;
        setter(path === "//" ? "/" : path);
    };

    const openPathSelector = (isSrcSelect) => {
        if (isSrcSelect) {
            if (
                !policies[options.policy] ||
                policies[options.policy].Type === "local" ||
                policies[options.policy].Type === "remote"
            ) {
                ToggleSnackbar(
                    "top",
                    "right",
                    "選擇的儲存策略只支援手動輸入路徑",
                    "warning"
                );
                return;
            }
            setSelectRemote(true);
        } else {
            if (user === null) {
                ToggleSnackbar("top", "right", "請先選擇目標使用者", "warning");
                return;
            }
            setSelectLocal(true);
        }
    };

    return (
        <div>
            <Dialog
                open={selectRemote}
                onClose={() => setSelectRemote(false)}
                aria-labelledby="form-dialog-title"
            >
                <DialogTitle id="form-dialog-title">選擇目錄</DialogTitle>
                <PathSelector
                    presentPath="/"
                    api={"/admin/file/folders/policy/" + options.policy}
                    selected={[]}
                    onSelect={setMoveTarget((p) =>
                        setOptions({
                            ...options,
                            src: p,
                        })
                    )}
                />

                <DialogActions>
                    <Button
                        onClick={() => setSelectRemote(false)}
                        color="primary"
                    >
                        確定
                    </Button>
                </DialogActions>
            </Dialog>
            <Dialog
                open={selectLocal}
                onClose={() => setSelectLocal(false)}
                aria-labelledby="form-dialog-title"
            >
                <DialogTitle id="form-dialog-title">選擇目錄</DialogTitle>
                <PathSelector
                    presentPath="/"
                    api={
                        "/admin/file/folders/user/" +
                        (user === null ? 0 : user.ID)
                    }
                    selected={[]}
                    onSelect={setMoveTarget((p) =>
                        setOptions({
                            ...options,
                            dst: p,
                        })
                    )}
                />

                <DialogActions>
                    <Button
                        onClick={() => setSelectLocal(false)}
                        color="primary"
                    >
                        確定
                    </Button>
                </DialogActions>
            </Dialog>
            <form onSubmit={submit}>
                <div className={classes.root}>
                    <Typography variant="h6" gutterBottom>
                        匯入外部目錄
                    </Typography>
                    <div className={classes.formContainer}>
                        <div className={classes.form}>
                            <Alert severity="info">
                                您可以將儲存策略中已有文件、目錄結構匯入到
                                Cloudreve
                                中，匯入操作不會額外占用物理儲存空間，但仍會正常扣除使用者已用容量空間，空間不足時將停止匯入。
                            </Alert>
                        </div>
                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    儲存策略
                                </InputLabel>
                                <Select
                                    labelId="demo-mutiple-chip-label"
                                    id="demo-mutiple-chip"
                                    value={options.policy}
                                    onChange={handleChange("policy")}
                                    input={<Input id="select-multiple-chip" />}
                                >
                                    {Object.keys(policies).map((pid) => (
                                        <MenuItem key={pid} value={pid}>
                                            {policies[pid].Name}
                                        </MenuItem>
                                    ))}
                                </Select>
                                <FormHelperText id="component-helper-text">
                                    選擇要匯入文件目前儲存所在的儲存策略
                                </FormHelperText>
                            </FormControl>
                        </div>
                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    目標使用者
                                </InputLabel>
                                <Input
                                    value={options.userInput}
                                    onChange={(e) => {
                                        handleChange("userInput")(e);
                                        setAnchorEl(e.currentTarget);
                                    }}
                                    startAdornment={
                                        user !== null && (
                                            <InputAdornment position="start">
                                                <Chip
                                                    size="small"
                                                    onDelete={() => {
                                                        setUser(null);
                                                    }}
                                                    label={user.Nick}
                                                />
                                            </InputAdornment>
                                        )
                                    }
                                    disabled={user !== null}
                                />
                                <Popper
                                    open={
                                        options.userInput !== "" &&
                                        users.length > 0
                                    }
                                    anchorEl={anchorEl}
                                    placement={"bottom"}
                                    transition
                                >
                                    {({ TransitionProps }) => (
                                        <Fade
                                            {...TransitionProps}
                                            timeout={350}
                                        >
                                            <Paper
                                                className={classes.userSelect}
                                            >
                                                {users.map((u) => (
                                                    <MenuItem
                                                        key={u.Email}
                                                        onClick={() =>
                                                            selectUser(u)
                                                        }
                                                    >
                                                        {u.Nick}{" "}
                                                        {"<" + u.Email + ">"}
                                                    </MenuItem>
                                                ))}
                                            </Paper>
                                        </Fade>
                                    )}
                                </Popper>
                                <FormHelperText id="component-helper-text">
                                    選擇要將文件匯入到哪個使用者的文件系統中，可透過暱稱、信箱搜尋使用者
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    原始目錄路徑
                                </InputLabel>

                                <Input
                                    value={options.src}
                                    onChange={(e) => {
                                        handleChange("src")(e);
                                        setAnchorEl(e.currentTarget);
                                    }}
                                    required
                                    endAdornment={
                                        <Button
                                            onClick={() =>
                                                openPathSelector(true)
                                            }
                                        >
                                            選擇
                                        </Button>
                                    }
                                />

                                <FormHelperText id="component-helper-text">
                                    要匯入的目錄在儲存端的路徑
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    目的目錄路徑
                                </InputLabel>

                                <Input
                                    value={options.dst}
                                    onChange={(e) => {
                                        handleChange("dst")(e);
                                        setAnchorEl(e.currentTarget);
                                    }}
                                    required
                                    endAdornment={
                                        <Button
                                            onClick={() =>
                                                openPathSelector(false)
                                            }
                                        >
                                            選擇
                                        </Button>
                                    }
                                />

                                <FormHelperText id="component-helper-text">
                                    要將目錄匯入到使用者文件系統中的路徑
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={options.recursive}
                                            onChange={handleCheckChange(
                                                "recursive"
                                            )}
                                        />
                                    }
                                    label="遞迴匯入子目錄"
                                />
                                <FormHelperText id="component-helper-text">
                                    是否將目錄下的所有子目錄遞迴匯入
                                </FormHelperText>
                            </FormControl>
                        </div>
                    </div>
                </div>

                <div className={classes.root}>
                    <Button
                        disabled={loading}
                        type={"submit"}
                        variant={"contained"}
                        color={"primary"}
                    >
                        建立匯入任務
                    </Button>
                </div>
            </form>
        </div>
    );
}
