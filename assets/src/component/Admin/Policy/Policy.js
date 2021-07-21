import Button from "@material-ui/core/Button";
import IconButton from "@material-ui/core/IconButton";
import Menu from "@material-ui/core/Menu";
import MenuItem from "@material-ui/core/MenuItem";
import Paper from "@material-ui/core/Paper";
import Select from "@material-ui/core/Select";
import { makeStyles } from "@material-ui/core/styles";
import Table from "@material-ui/core/Table";
import TableBody from "@material-ui/core/TableBody";
import TableCell from "@material-ui/core/TableCell";
import TableContainer from "@material-ui/core/TableContainer";
import TableHead from "@material-ui/core/TableHead";
import TablePagination from "@material-ui/core/TablePagination";
import TableRow from "@material-ui/core/TableRow";
import Tooltip from "@material-ui/core/Tooltip";
import { Delete, Edit } from "@material-ui/icons";
import React, { useCallback, useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { useHistory, useLocation } from "react-router";
import { toggleSnackbar } from "../../../actions";
import { policyTypeMap } from "../../../config";
import API from "../../../middleware/Api";
import { sizeToString } from "../../../utils";
import AddPolicy from "../Dialogs/AddPolicy";

const useStyles = makeStyles((theme) => ({
    root: {
        [theme.breakpoints.up("md")]: {
            marginLeft: 100,
        },
        marginBottom: 40,
    },
    content: {
        padding: theme.spacing(2),
    },
    container: {
        overflowX: "auto",
    },
    tableContainer: {
        marginTop: 16,
    },
    header: {
        display: "flex",
        justifyContent: "space-between",
    },
    headerRight: {},
}));

const columns = [
    { id: "#", label: "#", minWidth: 50 },
    { id: "name", label: "名稱", minWidth: 170 },
    { id: "type", label: "類型", minWidth: 170 },
    {
        id: "count",
        label: "下屬文件數",
        minWidth: 50,
        align: "right",
    },
    {
        id: "size",
        label: "資料量",
        minWidth: 100,
        align: "right",
    },
    {
        id: "action",
        label: "操作",
        minWidth: 170,
        align: "right",
    },
];

function useQuery() {
    return new URLSearchParams(useLocation().search);
}

export default function Policy() {
    const classes = useStyles();
    // const [loading, setLoading] = useState(false);
    // const [tab, setTab] = useState(0);
    const [policies, setPolicies] = useState([]);
    const [statics, setStatics] = useState([]);
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(10);
    const [total, setTotal] = useState(0);
    const [addDialog, setAddDialog] = useState(false);
    const [filter, setFilter] = useState("all");
    const [anchorEl, setAnchorEl] = React.useState(null);
    const [editID, setEditID] = React.useState(0);

    const location = useLocation();
    const history = useHistory();
    const query = useQuery();

    const handleClick = (event) => {
        setAnchorEl(event.currentTarget);
    };

    const handleClose = () => {
        setAnchorEl(null);
    };

    const dispatch = useDispatch();
    const ToggleSnackbar = useCallback(
        (vertical, horizontal, msg, color) =>
            dispatch(toggleSnackbar(vertical, horizontal, msg, color)),
        [dispatch]
    );

    useEffect(() => {
        if (query.get("code") === "0") {
            ToggleSnackbar("top", "right", "授權成功", "success");
        } else if (query.get("msg") && query.get("msg") !== "") {
            ToggleSnackbar(
                "top",
                "right",
                query.get("msg") + ", " + query.get("err"),
                "warning"
            );
        }
    }, [location]);

    const loadList = () => {
        API.post("/admin/policy/list", {
            page: page,
            page_size: pageSize,
            order_by: "id desc",
            conditions: filter === "all" ? {} : { type: filter },
        })
            .then((response) => {
                setPolicies(response.data.items);
                setStatics(response.data.statics);
                setTotal(response.data.total);
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            });
    };

    useEffect(() => {
        loadList();
    }, [page, pageSize, filter]);

    const deletePolicy = (id) => {
        API.delete("/admin/policy/" + id)
            .then(() => {
                loadList();
                ToggleSnackbar("top", "right", "儲存策略已刪除", "success");
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            });
    };

    const open = Boolean(anchorEl);

    return (
        <div>
            <AddPolicy open={addDialog} onClose={() => setAddDialog(false)} />
            <div className={classes.header}>
                <Button
                    color={"primary"}
                    onClick={() => setAddDialog(true)}
                    variant={"contained"}
                >
                    添加儲存策略
                </Button>
                <div className={classes.headerRight}>
                    <Select
                        style={{
                            marginRight: 8,
                        }}
                        value={filter}
                        onChange={(e) => setFilter(e.target.value)}
                    >
                        <MenuItem value={"all"}>全部</MenuItem>
                        <MenuItem value={"local"}>本機</MenuItem>
                        <MenuItem value={"remote"}>從機</MenuItem>
                        <MenuItem value={"qiniu"}>七牛</MenuItem>
                        <MenuItem value={"upyun"}>又拍雲</MenuItem>
                        <MenuItem value={"oss"}>阿里雲 OSS</MenuItem>
                        <MenuItem value={"cos"}>騰訊雲 COS</MenuItem>
                        <MenuItem value={"onedrive"}>OneDrive</MenuItem>
                        <MenuItem value={"s3"}>Amazon S3</MenuItem>
                    </Select>
                    <Button
                        color={"primary"}
                        onClick={() => loadList()}
                        variant={"outlined"}
                    >
                        重新整理
                    </Button>
                </div>
            </div>

            <Paper square className={classes.tableContainer}>
                <TableContainer className={classes.container}>
                    <Table aria-label="sticky table" size={"small"}>
                        <TableHead>
                            <TableRow style={{ height: 52 }}>
                                {columns.map((column) => (
                                    <TableCell
                                        key={column.id}
                                        align={column.align}
                                        style={{ minWidth: column.minWidth }}
                                    >
                                        {column.label}
                                    </TableCell>
                                ))}
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {policies.map((row) => (
                                <TableRow hover key={row.ID}>
                                    <TableCell>{row.ID}</TableCell>
                                    <TableCell>{row.Name}</TableCell>
                                    <TableCell>
                                        {policyTypeMap[row.Type] !==
                                            undefined &&
                                            policyTypeMap[row.Type]}
                                    </TableCell>
                                    <TableCell align={"right"}>
                                        {statics[row.ID] !== undefined &&
                                            statics[row.ID][0].toLocaleString()}
                                    </TableCell>
                                    <TableCell align={"right"}>
                                        {statics[row.ID] !== undefined &&
                                            sizeToString(statics[row.ID][1])}
                                    </TableCell>
                                    <TableCell align={"right"}>
                                        <Tooltip title={"刪除"}>
                                            <IconButton
                                                onClick={() =>
                                                    deletePolicy(row.ID)
                                                }
                                                size={"small"}
                                            >
                                                <Delete />
                                            </IconButton>
                                        </Tooltip>
                                        <Tooltip title={"編輯"}>
                                            <IconButton
                                                onClick={(e) => {
                                                    setEditID(row.ID);
                                                    handleClick(e);
                                                }}
                                                size={"small"}
                                            >
                                                <Edit />
                                            </IconButton>
                                        </Tooltip>
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </TableContainer>
                <TablePagination
                    rowsPerPageOptions={[10, 25, 100]}
                    component="div"
                    count={total}
                    rowsPerPage={pageSize}
                    page={page - 1}
                    onChangePage={(e, p) => setPage(p + 1)}
                    onChangeRowsPerPage={(e) => {
                        setPageSize(e.target.value);
                        setPage(1);
                    }}
                />
            </Paper>
            <Menu
                open={open}
                anchorEl={anchorEl}
                onClose={handleClose}
                keepMounted
            >
                <MenuItem
                    onClick={(e) => {
                        handleClose(e);
                        history.push("/admin/policy/edit/pro/" + editID);
                    }}
                >
                    專家模式編輯
                </MenuItem>
                <MenuItem
                    onClick={(e) => {
                        handleClose(e);
                        history.push("/admin/policy/edit/guide/" + editID);
                    }}
                >
                    嚮導模式編輯
                </MenuItem>
            </Menu>
        </div>
    );
}
